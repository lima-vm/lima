package hostagent

import (
	"context"
	"time"

	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/sshocker/pkg/ssh"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (a *HostAgent) waitForRequirements(ctx context.Context, label string, requirements []requirement) error {
	const (
		retries       = 60
		sleepDuration = 10 * time.Second
	)
	var mErr error

	for i, req := range requirements {
	retryLoop:
		for j := 0; j < retries; j++ {
			logrus.Infof("Waiting for the %s requirement %d of %d: %q", label, i+1, len(requirements), req.description)
			err := a.waitForRequirement(ctx, req)
			if err == nil {
				logrus.Infof("The %s requirement %d of %d is satisfied", label, i+1, len(requirements))
				break retryLoop
			}
			if j == retries-1 {
				mErr = multierror.Append(mErr,
					errors.Wrapf(err, "failed to satisfy the %s requirement %d of %d %q: %s",
						label, i+1, len(requirements), req.description, req.debugHint))
				break retryLoop
			}
			time.Sleep(10 * time.Second)
		}
	}
	return mErr
}

func (a *HostAgent) waitForRequirement(ctx context.Context, r requirement) error {
	logrus.Debugf("executing script %q", r.description)
	stdout, stderr, err := ssh.ExecuteScript("127.0.0.1", a.y.SSH.LocalPort, a.sshConfig, r.script, r.description)
	logrus.Debugf("stdout=%q, stderr=%q, err=%v", stdout, stderr, err)
	if err != nil {
		return errors.Wrapf(err, "stdout=%q, stderr=%q", stdout, stderr)
	}
	return nil
}

type requirement struct {
	description string
	script      string
	debugHint   string
}

func (a *HostAgent) essentialRequirements() []requirement {
	req := make([]requirement, 0)
	req = append(req, requirement{
		description: "ssh",
		script: `#!/bin/bash
true
`,
		debugHint: `Failed to SSH into the guest.
Make sure that the YAML field "ssh.localPort" is not used by other processes on the host.
If any private key under ~/.ssh is protected with a passphrase, you need to have ssh-agent to be running.
`,
	})
	if len(a.y.Mounts) > 0 {
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
			description: "/etc/fuse.conf to contain \"user_allow_other\"",
			script: `#!/bin/bash
set -eux -o pipefail
if ! timeout 30s bash -c "until grep -q ^user_allow_other /etc/fuse.conf; do sleep 3; done"; then
	echo >&2 "/etc/fuse.conf is not updated to contain \"user_allow_other\""
	exit 1
fi
`,
			debugHint: `Append "user_allow_other" to /etc/fuse.conf in the guest`,
		})

	}
	req = append(req, requirement{
		description: "the guest agent to be running",
		script: `#!/bin/bash
set -eux -o pipefail
sock="/run/user/$(id -u)/lima-guestagent.sock"
if ! timeout 30s bash -c "until [[ -S \"${sock}\" ]]; do sleep 3; done"; then
	echo >&2 "lima-guestagent is not installed yet"
	exit 1
fi
`,
		debugHint: `The guest agent (/run/user/$UID/lima-guestagent.sock) does not seem running.
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
		req = append(req, requirement{
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
