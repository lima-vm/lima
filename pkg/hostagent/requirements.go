// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

func (a *HostAgent) waitForRequirements(ctx context.Context, label string, requirements []requirement) error {
	const (
		retries       = 200
		sleepDuration = 3 * time.Second
	)
	var errs []error

	for i, req := range requirements {
		logrus.Infof("Waiting for the %s requirement %d of %d: %q", label, i+1, len(requirements), req.description)
	retryLoop:
		for j := range retries {
			err := a.waitForRequirement(ctx, req)
			if err == nil {
				logrus.Infof("The %s requirement %d of %d is satisfied", label, i+1, len(requirements))
				break retryLoop
			}
			if req.fatal {
				logrus.Infof("No further %s requirements will be checked", label)
				errs = append(errs, fmt.Errorf("failed to satisfy the %s requirement %d of %d %q: %s; skipping further checks: %w", label, i+1, len(requirements), req.description, req.debugHint, err))
				return errors.Join(errs...)
			}
			if j == retries-1 {
				errs = append(errs, fmt.Errorf("failed to satisfy the %s requirement %d of %d %q: %s: %w", label, i+1, len(requirements), req.description, req.debugHint, err))
				break retryLoop
			}
			select {
			case <-ctx.Done():
				errs = append(errs, fmt.Errorf("failed to satisfy the %s requirement %d of %d %q: %s: %w", label, i+1, len(requirements), req.description, req.debugHint, ctx.Err()))
				return errors.Join(errs...)
			case <-time.After(sleepDuration):
			}
		}
	}
	return errors.Join(errs...)
}

// prefixExportParam will modify a script to be executed by ssh.ExecuteScript so that it exports
// all the variables from /mnt/lima-cidata/param.env before invoking the actual interpreter.
//
//   - The script is executed in user mode, so needs to read the file using `sudo`.
//
//   - `sudo cat param.env | while …; do export …; done` does not work because the piping
//     creates a subshell, and the exported variables are not visible to the parent process.
//
//   - The `<<<"$string"` redirection is not available on alpine-lima, where /bin/bash is
//     just a wrapper around busybox ash.
//
// A script that will start with `#!/usr/bin/env ruby` will be modified to look like this:
//
//	while read -r line; do
//	    [ -n "$line" ] && export "$line"
//	done<<EOF
//	$(sudo cat /mnt/lima-cidata/param.env)
//	EOF
//	/usr/bin/env ruby
//
// ssh.ExecuteScript will strip the `#!` prefix from the first line and invoke the
// rest of the line as the command. The full script is then passed via STDIN. We use
// "$(printf '…')" to be able to use \n as newline escapes, to fit everything on a
// single line:
//
//	#!/bin/bash -c "$(printf 'while … done<<EOF\n$(sudo …)\nEOF\n/usr/bin/env ruby')"
//	#!/usr/bin/env ruby
//	…
//
// An earlier implementation used $'…' for quoting, but that isn't supported if the
// user switched the default shell to fish.
func prefixExportParam(script string, guestOS *limatype.OS) (string, error) {
	interpreter, err := ssh.ParseScriptInterpreter(script)
	if err != nil {
		return "", err
	}

	// TODO we should have a symbolic constant for `/mnt/lima-cidata`
	cidata := "/mnt/lima-cidata"
	sudo := "sudo"
	if guestOS != nil && *guestOS == limatype.DARWIN {
		cidata = "/Volumes/cidata"
		// On macOS, /Volumes/cidata is not mounted as "root access only".
		// FIXME: The cidata does not need to be root-only on Linux, either?
		sudo = ""
	}
	exportParam := `param_env="$(` + sudo + ` cat ` + cidata + `/param.env)"; while read -r line; do [ -n "$line" ] && export "$line"; done<<EOF\n${param_env}\nEOF\n`

	// double up all '%' characters so we can pass them through unchanged in the format string of printf
	interpreter = strings.ReplaceAll(interpreter, "%", "%%")
	exportParam = strings.ReplaceAll(exportParam, "%", "%%")
	// strings will be interpolated into single-quoted strings, so protect any existing single quotes
	interpreter = strings.ReplaceAll(interpreter, "'", `'"'"'`)
	exportParam = strings.ReplaceAll(exportParam, "'", `'"'"'`)
	return fmt.Sprintf("#!/bin/bash -c \"$(printf '%s%s')\"\n%s", exportParam, interpreter, script), nil
}

func (a *HostAgent) bashAvailable() bool {
	return *a.instConfig.OS != limatype.FREEBSD
}

func (a *HostAgent) waitForRequirement(ctx context.Context, r requirement) error {
	if r.check != nil {
		logrus.Debugf("checking requirement %q", r.description)
		return r.check(ctx)
	}
	logrus.Debugf("executing script %q", r.description)
	script := r.script
	if a.bashAvailable() {
		var err error
		// FIXME: prefixExportParam depends on bash
		script, err = prefixExportParam(r.script, a.instConfig.OS)
		if err != nil {
			return err
		}
	}
	sshConfig := a.sshConfig
	if r.noMaster || runtime.GOOS == "windows" {
		// Remove ControlMaster, ControlPath, and ControlPersist options,
		// because Cygwin-based SSH clients do not support multiplexing when executing commands.
		// References:
		//   https://inbox.sourceware.org/cygwin/c98988a5-7e65-4282-b2a1-bb8e350d5fab@acm.org/T/
		//   https://stackoverflow.com/questions/20959792/is-ssh-controlmaster-with-cygwin-on-windows-actually-possible
		// By removing these options:
		//   - Avoids execution failures when the control master is not yet available.
		//   - Prevents error messages such as:
		//     > mux_client_request_session: read from master failed: Connection reset by peer
		//     > ControlSocket ....sock already exists, disabling multiplexing
		//     > mm_send_fd: sendmsg(2): Connection reset by peer\\r\\nmux_client_request_session: send fds failed\\r\\n
		sshConfig = &ssh.SSHConfig{
			ConfigFile:     sshConfig.ConfigFile,
			Persist:        false,
			AdditionalArgs: sshutil.DisableControlMasterOptsFromSSHArgs(sshConfig.AdditionalArgs),
		}
	}
	stdout, stderr, err := ssh.ExecuteScript(a.instSSHAddress, a.sshLocalPort, sshConfig, script, r.description)
	logrus.Debugf("stdout=%q, stderr=%q, err=%v", stdout, stderr, err)
	if err != nil {
		return fmt.Errorf("stdout=%q, stderr=%q: %w", stdout, stderr, err)
	}
	return nil
}

// requirement describes one readiness check performed before Ready fires.
// If check is non-nil, it is invoked directly and script is ignored;
// otherwise script is executed in the guest via ssh.ExecuteScript.
type requirement struct {
	description string
	script      string
	check       func(ctx context.Context) error
	debugHint   string
	fatal       bool
	noMaster    bool
}

func (a *HostAgent) essentialRequirements() []requirement {
	req := make([]requirement, 0)
	// Run a host-side TCP probe before the script-based "ssh" requirement
	// below. The script path uses Lima's own SSH client, which retries
	// internally and so can mask a race where the host agent accepts on the
	// forwarded port before guest sshd has bound :22. External clients
	// (ssh-keyscan, sftp, rsync, ...) that connect during that window get
	// EPIPE on their first write.
	logLocation := "/var/log/cloud-init-output.log in the guest"
	if a.instConfig.OS != nil && *a.instConfig.OS == limatype.DARWIN {
		logLocation = "serialv.log in the host"
	}
	addr := a.instSSHAddress
	port := a.sshLocalPort
	req = append(req,
		requirement{
			description: "sshLocalPort serves an SSH banner",
			check: func(ctx context.Context) error {
				return probeSSHBannerOnLocalPort(ctx, addr, port)
			},
			debugHint: fmt.Sprintf(`The host agent is listening on %s but the proxied
connection into the guest is failing. Most commonly this means guest
sshd has not yet bound :22. Check %s for sshd startup.
`, net.JoinHostPort(addr, strconv.Itoa(port)), logLocation),
		})
	req = append(req,
		requirement{
			description: "ssh",
			script: `#!/bin/sh
true
`,
			debugHint: `Failed to SSH into the guest.
Make sure that the YAML field "ssh.localPort" is not used by other processes on the host.
If any private key under ~/.ssh is protected with a passphrase, you need to have ssh-agent to be running.
`,
			noMaster: true,
		})
	startControlMasterReq := requirement{
		description: "Explicitly start ssh ControlMaster",
		script: `#!/bin/sh
true
`,
		debugHint: `The persistent ssh ControlMaster should be started immediately.`,
	}
	if *a.instConfig.Plain || *a.instConfig.OS != limatype.LINUX {
		req = append(req, startControlMasterReq)
		return req
	}
	req = append(req,
		requirement{
			description: "user session is ready for ssh",
			script: fmt.Sprintf(`#!/bin/sh
set -eux
[ "$(cat /run/lima-ssh-ready 2>/dev/null)" = "%s" ]
`, a.iid),
			debugHint: `The boot sequence will terminate any existing user session after updating
/etc/environment to make sure the session includes the new values.
Terminating the session will break the persistent SSH tunnel, so
it must not be created until the session reset is done.
`,
			noMaster: true,
		})

	if *a.instConfig.MountType == limatype.REVSSHFS && len(a.instConfig.Mounts) > 0 {
		req = append(req, requirement{
			description: "sshfs binary to be installed",
			script: `#!/bin/sh
set -eux
command -v sshfs
`,
			debugHint: `The sshfs binary was not installed in the guest.
Make sure that you are using an officially supported image.
Also see "/var/log/cloud-init-output.log" in the guest.
A possible workaround is to run "apt-get install sshfs" in the guest.
`,
		})
		req = append(req, requirement{
			description: "fuse to \"allow_other\" as user",
			script: `#!/bin/sh
set -eux
sudo grep -q ^user_allow_other /etc/fuse*.conf
`,
			debugHint: `Append "user_allow_other" to /etc/fuse.conf (/etc/fuse3.conf) in the guest`,
		})
	} else {
		req = append(req, startControlMasterReq)
	}
	return req
}

func (a *HostAgent) optionalRequirements() []requirement {
	req := make([]requirement, 0)
	isLinuxGuest := a.instConfig.OS == nil || *a.instConfig.OS == limatype.LINUX
	if isLinuxGuest && (*a.instConfig.Containerd.System || *a.instConfig.Containerd.User) && !*a.instConfig.Plain {
		req = append(req,
			requirement{
				description: "systemd must be available",
				fatal:       true,
				script: `#!/bin/bash
set -eux -o pipefail
if ! command -v systemctl 2>&1 >/dev/null; then
    echo >&2 "systemd is not available on this OS"
    exit 1
fi
`,
				debugHint: `systemd is required to run containerd, but does not seem to be available.
Make sure that you use an image that supports systemd. If you do not want to run
containerd, please make sure that both 'container.system' and 'containerd.user'
are set to 'false' in the config file.
`,
			},
			requirement{
				description: "containerd binaries to be installed",
				script: `#!/bin/sh
set -eux
command -v nerdctl || test -x ` + *a.instConfig.GuestInstallPrefix + `/bin/nerdctl
`,
				debugHint: `The nerdctl binary was not installed in the guest.
Make sure that you are using an officially supported image.
Also see "/var/log/cloud-init-output.log" in the guest.
`,
			})
	}
	for _, probe := range a.instConfig.Probes {
		if probe.Mode == limatype.ProbeModeReadiness {
			req = append(req, requirement{
				description: probe.Description,
				script:      *probe.Script,
				debugHint:   probe.Hint,
			})
		}
	}
	return req
}

func (a *HostAgent) finalRequirements() []requirement {
	req := make([]requirement, 0)
	logLocation := "/var/log/cloud-init-output.log in the guest"
	if *a.instConfig.OS == limatype.DARWIN {
		logLocation = "serialv.log in the host"
	}
	req = append(req,
		requirement{
			description: "boot scripts must have finished",
			script: fmt.Sprintf(`#!/bin/sh
set -eux
BOOT_DONE=/run/lima-boot-done
UNAME="$(uname)"
if [ "$UNAME" = "Darwin" ] || [ "$UNAME" = "FreeBSD" ]; then
	BOOT_DONE=/var/run/lima-boot-done
fi
[ "$(cat "$BOOT_DONE" 2>/dev/null)" = "%s" ]
`, a.iid),
			debugHint: `All boot scripts, provisioning scripts, and readiness probes must
finish before the instance is considered "ready".
Check "` + logLocation + `" to see where the process is blocked!
`,
		})
	return req
}

const (
	probeSSHBannerTimeout = 5 * time.Second
	// RFC 4253 §4.2 allows the server to emit lines before the identification
	// string. Cap the read so a misbehaving peer can't loop forever.
	probeSSHBannerMaxLines = 50
)

// probeSSHBannerOnLocalPort dials <address>:<port> and returns nil when a
// line starting with "SSH-" is read within probeSSHBannerMaxLines.
func probeSSHBannerOnLocalPort(ctx context.Context, address string, port int) error {
	addr := net.JoinHostPort(address, strconv.Itoa(port))
	d := net.Dialer{Timeout: probeSSHBannerTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()
	// Propagate ctx cancellation to the in-flight read.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.SetReadDeadline(time.Now().Add(probeSSHBannerTimeout)); err != nil {
		return fmt.Errorf("set read deadline on %s: %w", addr, err)
	}
	r := bufio.NewReader(conn)
	for range probeSSHBannerMaxLines {
		line, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read SSH banner from %s: %w", addr, err)
		}
		if strings.HasPrefix(line, "SSH-") {
			return nil
		}
	}
	return fmt.Errorf("no SSH identification line from %s within %d lines", addr, probeSSHBannerMaxLines)
}
