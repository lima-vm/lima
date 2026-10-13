# Lima threat model

Anthropic's OSS Scanner reads this file before it scans Lima. Lima's own statement of its security
model is the [CNCF self-assessment](https://github.com/cncf/tag-security/blob/main/community/assessments/projects/lima/self-assessment.md);
this file summarizes it and adds what a scanner needs.

## What Lima does and where untrusted input enters

Lima runs Linux virtual machines, and experimentally macOS, FreeBSD and Windows ones, for one user
on a macOS, Linux or Windows host, with automatic file sharing and port forwarding. `limactl`
creates and manages instances without root on the host. For each running instance, a host agent
(`limactl hostagent`) drives the VM through a driver (QEMU, Virtualization.framework, WSL2, HCS or
krunkit), forwards ports, sets up mounts and answers the guest's DNS queries. A guest agent
(`lima-guestagent`) inside the VM reports listening ports and other events to the host agent.

Attacker-controlled:

- **Everything in the guest, including root.** Users run untrusted code in their VMs, and the docs
  recommend Lima for keeping AI agents away from host files. The guest user has passwordless sudo
  by default, so assume the attacker is guest root. Guest data reaches the host through guest agent
  events and port-forwarding streams (`pkg/guestagent/api`, `pkg/hostagent`, `pkg/portfwd`,
  `pkg/portfwdserver`), DNS queries (`pkg/hostagent/dns`), the `user-v2` network stack
  (`pkg/networks/usernet`, built on gvisor-tap-vsock), the host's sftp-server for `reverse-sshfs`
  mounts, and files that `limactl copy` and `limactl shell --sync` bring back to the host.
- **Disk images and other downloads.** `pkg/downloader` fetches images and archives and checks
  their digests when the template gives one. `pkg/imgutil` and `pkg/qemuimgutil` inspect and
  convert images on the host. A crafted image must not make the host read or write other host
  files.
- **Other local users.** An instance directory under `~/.lima` holds control sockets (`ha.sock`,
  `ssh.sock`, `ga.sock`, `qmp.sock`), and `~/.lima/_config/user` is the private key that logs into
  every guest. `limactl sudoers` prints a sudoers file for the user to install. It lets the group
  named in `networks.yaml` run the network helpers as root (`socket_vmnet` on macOS,
  `lima-privileged-net` on Linux), and on macOS it lets the current user run
  `lima-privileged-block-device` for the devices listed. Whoever a grant covers must gain nothing
  beyond the helper's job.
- **Unprivileged guest users, when `user.passwordlessSudo: false`.** Guest root is then a boundary
  inside the guest.

Trusted: templates and `lima.yaml`, including `base` URLs, provisioning scripts, probes, mounts,
port-forward rules and `param` values; `networks.yaml` once it passes validation; the user's
command line, environment and `LIMA_HOME`; and the VM drivers themselves.

## Components that matter most / least

Most:

- Host agent code that parses or acts on guest data: events and port forwarding
  (`pkg/hostagent/port*.go`, `pkg/portfwd`, `pkg/portfwdserver`), the DNS server and `user-v2`.
  Port-forward rules decide which host addresses a guest port appears on, and the default is
  127.0.0.1 only.
- The privileged helpers and their sudoers rules: `cmd/lima-privileged-net`,
  `cmd/lima-privileged-block-device`, `pkg/blockdevice`, `pkg/networks` (`validate.go`,
  `sudoers.go`, `reconcile`), `pkg/sudoers` and `cmd/limactl/sudoers*.go`.
  `pkg/networks/networks.TEMPLATE.yaml` states their invariants. Only root may modify a helper or
  its parent directories, the user must not be able to write `varRun` (helpers are killed through
  sudo by the pid files in it), and no path segment may be a symlink.
- Image handling: `pkg/downloader`, `pkg/imgutil` and `pkg/qemuimgutil`, including the checks that
  reject backing files, qcow2 external data files and VMDK extents, which point at other host files.
- Moving files between guest and host: mounts (`pkg/hostagent/mount.go`), `pkg/copytool`,
  `limactl shell --sync`, and the MCP tools (`cmd/limactl-mcp`, `pkg/mcp`), which must act only
  inside the guest.

Least: `hack/`, `website/`, test fixtures, and the image URLs in `templates/`. Code for macOS
hosts (vz, krunkit, block devices) and Windows hosts (WSL2, HCS) is in scope for review but cannot
run in this image; mark findings there as not executed.

## How to exercise it

- `/src` is the checkout, built with `make DEBUG=1 native` (symbols and DWARF kept). `make install`
  put `limactl`, `lima`, the guest agent, the templates and the helpers under `/usr/local`. Go's
  module and build caches are warm, so `go build ./...` and `go test ./...` work offline.
- `limactl` refuses to run as root. Run it as the unprivileged `lima` user, for example
  `runuser -u lima -- limactl list`.
- We expect this machine to provide nested KVM. Lima uses KVM whenever `/dev/kvm` exists, and QEMU
  fails when the `lima` user cannot open it, so run `chmod 666 /dev/kvm` first. Without
  `/dev/kvm`, QEMU emulates the CPU and the boot takes several minutes.
- The `lima` user has an instance named `default`. It runs Ubuntu from `template:default` and
  rootless containerd, and mounts `/home/lima` read-only over 9p. The build booted it once, so
  `runuser -u lima -- limactl start default` starts it offline. `limactl shell default` gives a
  guest shell, and `sudo` in it gives guest root.
- The download cache holds that Ubuntu image, so a new instance from `template:default` also
  starts offline. Use one to try other settings, such as `user.passwordlessSudo: false` or the
  `user-v2` network. A new instance's first boot cannot install `uidmap`, which rootless
  containerd needs; `limactl clone default NAME` gives a copy that has it.
- Go tests can stand in for the guest, as the tests in `pkg/hostagent`, `pkg/portfwd` and
  `pkg/networks/usernet` do. To attack `user-v2` with crafted packets, hand one end of a
  `net.Pipe` to gvisor-tap-vsock's `virtualnetwork.AcceptQemu` and write Ethernet frames to the
  other.
- The tests run as root here, so a test that expects a permission error may fail. That is not a
  finding.

## How we rate severity

- Critical: code execution on the host, or reading or writing host files outside the configured
  mounts, driven from inside the guest; root on the host for a user who is not already root,
  through the helpers or their sudoers rules.
- High: a guest or another local user taking over an instance's host-side resources (the SSH key,
  the sockets); a host port reachable beyond localhost that the configuration did not ask for; a
  guest writing through a read-only mount; an unprivileged guest user gaining guest root when
  passwordless sudo is off (GHSA-2j9v-p4xj-cjw2 was rated high).
- Medium: a guest crashing or hanging `limactl` or the host agent, or making it use unbounded memory
  or disk.
- Low: a crafted disk image reading a host file (GHSA-f7qw-jj9c-rpq9 was rated low); hardening gaps
  with no demonstrated exploit.

## Anything to leave alone

- Anything a template can do. Provisioning scripts run as root in the guest, mounts expose host
  files and port-forward rules publish ports. The self-assessment lists tolerance of malicious
  templates as a non-goal, and we apply that to templates fetched from a URL too.
- The guest reading host files under a configured mount. The default template mounts the user's
  home directory read-only.
- The guest reaching host files outside a `reverse-sshfs` mount, an exception to the Critical
  rating above. Lima serves the mount with OpenSSH's `sftp-server` or a builtin sftp server, and
  neither confines the guest to the mount directory. The
  [mount docs](https://lima-vm.io/docs/config/mount/#reverse-sshfs) list this caveat. Writing
  through a read-only mount is still a finding, since Lima starts either server read-only for a
  read-only mount.
- The guest reaching services on the host's loopback addresses at `host.lima.internal`
  (192.168.5.2). User-mode networking does that by design.
- The guest user being root through the default passwordless sudo.
- Guest ports appearing on the host's localhost. That is port forwarding.
- Bugs in QEMU, Virtualization.framework, WSL2, Hyper-V, OpenSSH or the guest images, and outdated
  images in templates.
- Anything that needs control of the user's `LIMA_HOME`, `PATH` or environment, or root on the
  host.

## How reports should look

Name the attacker (a guest process, guest root, another local user, an image publisher), what they
control, the host OS and VM driver, and the path from input to impact. A Go test that fails before
the fix and passes after it is the best reproducer. Keep patches small.
