package hostagent

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/sshocker/pkg/ssh"
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
			a.l.Infof("Waiting for the %s requirement %d of %d: %q", label, i+1, len(requirements), req.description)
			err := a.waitForRequirement(ctx, req)
			if err == nil {
				a.l.Infof("The %s requirement %d of %d is satisfied", label, i+1, len(requirements))
				break retryLoop
			}
			if req.fatal {
				a.l.Infof("No further %s requirements will be checked", label)
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

func (a *HostAgent) waitForRequirement(ctx context.Context, r requirement) error {
	a.l.Debugf("executing script %q", r.description)
	stdout, stderr, err := ssh.ExecuteScript("127.0.0.1", a.y.SSH.LocalPort, a.sshConfig, r.script, r.description)
	a.l.Debugf("stdout=%q, stderr=%q, err=%v", stdout, stderr, err)
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
			description: "/sbin/mount.cifs to be installed",
			script: `#!/bin/bash
set -eux -o pipefail
ls -l /sbin/mount.cifs
`,
			debugHint: "/sbin/mount.cifs does not seem installed. See \"/var/log/cloud-init-output.log\" in the guest.",
		})
	}

	for i, m := range a.y.Mounts {
		locationExpanded, err := localpathutil.Expand(m.Location)
		if err != nil {
			panic(err) // should have been already validated
		}
		description := fmt.Sprintf("directory %s to be mounted (writable: %v)", locationExpanded, m.Writable)
		if i == 0 {
			description += " [May take 5-10 seconds]"
		}
		req = append(req, requirement{
			description: description,
			script: fmt.Sprintf(`#!/bin/bash
set -eux -o pipefail
# FIXME: not robust
grep %q /proc/mounts
`, locationExpanded),
			debugHint: fmt.Sprintf("The directory %q does not seem mounted. See \"/var/log/cloud-init-output.log\" in the guest.", locationExpanded),
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
