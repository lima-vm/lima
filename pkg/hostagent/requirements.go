package hostagent

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

func (a *HostAgent) waitForRequirements(label string, requirements []requirement) error {
	const (
		retries       = 60
		sleepDuration = 10 * time.Second
	)
	var mErr error

	for i, req := range requirements {
	retryLoop:
		for j := 0; j < retries; j++ {
			logrus.Infof("Waiting for the %s requirement %d of %d: %q", label, i+1, len(requirements), req.description)
			err := a.waitForRequirement(req)
			if err == nil {
				logrus.Infof("The %s requirement %d of %d is satisfied", label, i+1, len(requirements))
				break retryLoop
			}
			if req.fatal {
				logrus.Infof("No further %s requirements will be checked", label)
				return multierror.Append(mErr, fmt.Errorf("failed to satisfy the %s requirement %d of %d %q: %s; skipping further checks: %w", label, i+1, len(requirements), req.description, req.debugHint, err))
			}
			if j == retries-1 {
				mErr = multierror.Append(mErr, fmt.Errorf("failed to satisfy the %s requirement %d of %d %q: %s: %w", label, i+1, len(requirements), req.description, req.debugHint, err))
				break retryLoop
			}
			time.Sleep(10 * time.Second)
		}
	}
	return mErr
}

func (a *HostAgent) waitForRequirement(r requirement) error {
	logrus.Debugf("executing script %q", r.description)
	stdout, stderr, err := ssh.ExecuteScript("127.0.0.1", a.sshLocalPort, a.sshConfig, r.script, r.description)
	logrus.Debugf("stdout=%q, stderr=%q, err=%v", stdout, stderr, err)
	if err != nil {
		return fmt.Errorf("stdout=%q, stderr=%q: %w", stdout, stderr, err)
	}
	return nil
}

type requirement struct {
	description string
	script      string
	debugHint   string
	fatal       bool
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
		},
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
		})

	if *a.y.MountType == limayaml.REVSSHFS && len(a.y.Mounts) > 0 {
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
			description: "/etc/fuse.conf (/etc/fuse3.conf) to contain \"user_allow_other\"",
			script: `#!/bin/bash
set -eux -o pipefail
if ! timeout 30s bash -c "until grep -q ^user_allow_other /etc/fuse*.conf; do sleep 3; done"; then
	echo >&2 "/etc/fuse.conf (/etc/fuse3.conf) is not updated to contain \"user_allow_other\""
	exit 1
fi
`,
			debugHint: `Append "user_allow_other" to /etc/fuse.conf (/etc/fuse3.conf) in the guest`,
		})

	}
	req = append(req, requirement{
		description: "the guest agent to be running",
		script: `#!/bin/bash
set -eux -o pipefail
sock="/run/lima-guestagent.sock"
if ! timeout 30s bash -c "until [ -S \"${sock}\" ]; do sleep 3; done"; then
	echo >&2 "lima-guestagent is not installed yet"
	exit 1
fi
`,
		debugHint: `The guest agent (/run/lima-guestagent.sock) does not seem running.
Make sure that you are using an officially supported image.
Also see "/var/log/cloud-init-output.log" in the guest.
A possible workaround is to run "lima-guestagent install-systemd" in the guest.
`,
	})
	return req
}

func (a *HostAgent) optionalRequirements() []requirement {
	req := make([]requirement, 0)
	if *a.y.Containerd.System || *a.y.Containerd.User {
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
if ! timeout 30s bash -c "until command -v nerdctl; do sleep 3; done"; then
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
	for _, probe := range a.y.Probes {
		if probe.Mode == limayaml.ProbeModeReadiness {
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
