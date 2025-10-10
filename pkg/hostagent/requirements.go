// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

func (a *HostAgent) waitForRequirements(label string, requirements []requirement) error {
	const (
		retries       = 60
		sleepDuration = 10 * time.Second
	)
	var errs []error

	for i, req := range requirements {
	retryLoop:
		for j := range retries {
			logrus.Infof("Waiting for the %s requirement %d of %d: %q", label, i+1, len(requirements), req.description)
			err := a.waitForRequirement(req)
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
			time.Sleep(10 * time.Second)
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
func prefixExportParam(script string) (string, error) {
	interpreter, err := ssh.ParseScriptInterpreter(script)
	if err != nil {
		return "", err
	}

	// TODO we should have a symbolic constant for `/mnt/lima-cidata`
	exportParam := `while read -r line; do [ -n "$line" ] && export "$line"; done<<EOF\n$(sudo cat /mnt/lima-cidata/param.env)\nEOF\n`

	// double up all '%' characters so we can pass them through unchanged in the format string of printf
	interpreter = strings.ReplaceAll(interpreter, "%", "%%")
	exportParam = strings.ReplaceAll(exportParam, "%", "%%")
	// strings will be interpolated into single-quoted strings, so protect any existing single quotes
	interpreter = strings.ReplaceAll(interpreter, "'", `'"'"'`)
	exportParam = strings.ReplaceAll(exportParam, "'", `'"'"'`)
	return fmt.Sprintf("#!/bin/bash -c \"$(printf '%s%s')\"\n%s", exportParam, interpreter, script), nil
}

func (a *HostAgent) waitForRequirement(r requirement) error {
	logrus.Debugf("executing script %q", r.description)
	script, err := prefixExportParam(r.script)
	if err != nil {
		return err
	}
	sshConfig := a.sshConfig
	if r.noMaster {
		sshConfig = &ssh.SSHConfig{
			ConfigFile:     sshConfig.ConfigFile,
			Persist:        false,
			AdditionalArgs: sshutil.DisableControlMasterOptsFromSSHArgs(sshConfig.AdditionalArgs),
		}
	}
	sshAddress, sshPort := a.sshAddressPort()
	stdout, stderr, err := ssh.ExecuteScript(sshAddress, sshPort, sshConfig, script, r.description)
	logrus.Debugf("stdout=%q, stderr=%q, err=%v", stdout, stderr, err)
	if err != nil {
		return fmt.Errorf("stdout=%q, stderr=%q: %w", stdout, stderr, err)
	}
	if r.stdoutParser != nil {
		return r.stdoutParser(stdout)
	}
	return nil
}

type requirement struct {
	description  string
	script       string
	debugHint    string
	fatal        bool
	noMaster     bool
	stdoutParser func(string) error
}

func (a *HostAgent) essentialRequirements() []requirement {
	req := make([]requirement, 0)
	req = append(req,
		requirement{
			description: "ssh",
			script: `#!/bin/bash
true
`,
			debugHint: `Failed to SSH into the guest.
Make sure that the YAML field "ssh.localPort" is not used by other processes on the host.
If any private key under ~/.ssh is protected with a passphrase, you need to have ssh-agent to be running.
`,
			noMaster: true,
		},
	)
	if runtime.GOOS == "darwin" {
		// Limit the Guest IP address detection only to macOS for now.
		req = append(req,
			requirement{
				description: "detect guest interface on same subnet as the host",
				script: `#!/bin/bash
ip -j neighbor
`,
				debugHint: `Detecting the guest has interface in same subnet on the host.
This is only supported on macOS for now.
If the guest does not have interface in same subnet on the host, SSH connection against the guest OS will be made via the localhost port forwarding.`,
				noMaster:     true,
				stdoutParser: a.detectGuestIfnameOnSameSubnetAtHost,
			},
			requirement{
				description: "detect guest IP address",
				script: `#!/bin/bash
ip -j addr
`,
				debugHint: `Detecting the guest IP address on the interface in same subnet on the host.
This is only supported on macOS for now.
If the interface does not have IPv4 address, SSH connection against the guest OS will be made via the localhost port forwarding.`,
				noMaster:     true,
				stdoutParser: a.detectGuestIPAddress,
			},
		)
	}
	startControlMasterReq := requirement{
		description: "Explicitly start ssh ControlMaster",
		script: `#!/bin/bash
true
`,
		debugHint: `The persistent ssh ControlMaster should be started immediately.`,
	}
	if *a.instConfig.Plain {
		req = append(req, startControlMasterReq)
		return req
	}
	req = append(req,
		requirement{
			description: "user session is ready for ssh",
			script: `#!/bin/bash
set -eux -o pipefail
if ! timeout 30s bash -c "until sudo diff -q /run/lima-ssh-ready /mnt/lima-cidata/meta-data 2>/dev/null; do sleep 3; done"; then
	echo >&2 "not ready to start persistent ssh session"
	exit 1
fi
`,
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
			script: `#!/bin/bash
set -eux -o pipefail
if ! timeout 30s bash -c "until command -v sshfs; do sleep 3; done"; then
	echo >&2 "sshfs is not installed yet"
	exit 1
fi
`,
			debugHint: `The sshfs binary was not installed in the guest.
Make sure that you are using an officially supported image.
Also see "/var/log/cloud-init-output.log" in the guest.
A possible workaround is to run "apt-get install sshfs" in the guest.
`,
		})
		req = append(req, requirement{
			description: "fuse to \"allow_other\" as user",
			script: `#!/bin/bash
set -eux -o pipefail
if ! timeout 30s bash -c "until sudo grep -q ^user_allow_other /etc/fuse*.conf; do sleep 3; done"; then
	echo >&2 "/etc/fuse.conf (/etc/fuse3.conf) is not updated to contain \"user_allow_other\""
	exit 1
fi
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
	if (*a.instConfig.Containerd.System || *a.instConfig.Containerd.User) && !*a.instConfig.Plain {
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
				script: `#!/bin/bash
set -eux -o pipefail
if ! timeout 30s bash -c "until command -v nerdctl || test -x ` + *a.instConfig.GuestInstallPrefix + `/bin/nerdctl; do sleep 3; done"; then
	echo >&2 "nerdctl is not installed yet"
	exit 1
fi
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
				script:      probe.Script,
				debugHint:   probe.Hint,
			})
		}
	}
	return req
}

func (a *HostAgent) finalRequirements() []requirement {
	req := make([]requirement, 0)
	req = append(req,
		requirement{
			description: "boot scripts must have finished",
			script: `#!/bin/bash
set -eux -o pipefail
if ! timeout 30s bash -c "until sudo diff -q /run/lima-boot-done /mnt/lima-cidata/meta-data 2>/dev/null; do sleep 3; done"; then
	echo >&2 "boot scripts have not finished"
	exit 1
fi
`,
			debugHint: `All boot scripts, provisioning scripts, and readiness probes must
finish before the instance is considered "ready".
Check "/var/log/cloud-init-output.log" in the guest to see where the process is blocked!
`,
		})
	return req
}

// detectGuestIfnameOnSameSubnetAtHost detects the guest interface name on the same subnet on the host
// by comparing the MAC addresses of the host network interfaces and the output of "ip -j neighbor" command in the guest.
func (a *HostAgent) detectGuestIfnameOnSameSubnetAtHost(stdout string) error {
	var neighbors []struct {
		DST    string `json:"dst"`
		DEV    string `json:"dev"`
		LLADDR string `json:"lladdr"`
	}
	if err := json.Unmarshal([]byte(stdout), &neighbors); err != nil {
		return fmt.Errorf("failed to parse ip neighbor output %q: %w", stdout, err)
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("failed to get network interfaces: %w", err)
	}
	for _, neighbor := range neighbors {
		for _, ifi := range interfaces {
			if ifi.HardwareAddr.String() != neighbor.LLADDR {
				continue
			}
			a.guestIPMu.Lock()
			a.guestIfnameOnSameSubnetAsHost = neighbor.DEV
			a.guestIPMu.Unlock()
			logrus.Infof("Detected the guest has interface %q in same subnet on the host", neighbor.DEV)
			return nil
		}
	}
	logrus.Info("The guest does not have interface in same subnet on the host")
	return nil
}

// detectGuestIPAddress detects the guest IP address on the interface in same subnet on the host
// by parsing the output of "ip -j addr" command in the guest.
func (a *HostAgent) detectGuestIPAddress(stdout string) error {
	a.guestIPMu.RLock()
	guestIfnameOnSameSubnetAsHost := a.guestIfnameOnSameSubnetAsHost
	a.guestIPMu.RUnlock()
	if guestIfnameOnSameSubnetAsHost == "" {
		return nil
	}
	var addrs []struct {
		IFNAME string `json:"ifname"`
		ADDRS  []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
			Scope  string `json:"scope"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal([]byte(stdout), &addrs); err != nil {
		return fmt.Errorf("failed to parse ip addr output %q: %w", stdout, err)
	}
	var (
		guestIPv4 net.IP
		guestIPv6 net.IP
	)
	for _, addr := range addrs {
		if addr.IFNAME == guestIfnameOnSameSubnetAsHost {
			for _, addr := range addr.ADDRS {
				if addr.Scope != "global" {
					continue
				}
				switch addr.Family {
				case "inet":
					guestIPv4 = net.ParseIP(addr.Local)
				case "inet6":
					guestIPv6 = net.ParseIP(addr.Local)
				}
			}
		}
	}
	if guestIPv4 == nil && guestIPv6 == nil {
		logrus.Infof("The interface %q has neither an IPv4 nor an IPv6 address", guestIfnameOnSameSubnetAsHost)
		return nil
	}
	if guestIPv4 != nil {
		logrus.Infof("The guest IPv4 address on the interface %q is %q", guestIfnameOnSameSubnetAsHost, guestIPv4)
	}
	if guestIPv6 != nil {
		logrus.Infof("The guest IPv6 address on the interface %q is %q", guestIfnameOnSameSubnetAsHost, guestIPv6)
	}
	a.guestIPMu.Lock()
	a.guestIPv4 = guestIPv4
	a.guestIPv6 = guestIPv6
	a.guestIPMu.Unlock()
	ctx := context.Background()
	return a.WriteSSHConfigFile(ctx)
}
