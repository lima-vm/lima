---
relations:
  - entity: snapshot
    verb: has
    cardinality: one-to-many
  - entity: instance-event
    verb: records
    cardinality: one-to-many
  - entity: lima-disk
    verb: attaches
    cardinality: many-to-many
  - entity: network
    verb: joins
    cardinality: many-to-many
references:
  - kind: code
    role: implementation
    target: pkg/limatype/lima_instance.go#Instance
  - kind: code
    role: implementation
    target: pkg/store/instance.go#Inspect
  - kind: code
    role: implementation
    target: pkg/limatype/lima_yaml.go#LimaYAML
  - kind: doc
    role: context
    target: templates/default.yaml
  - kind: doc
    role: context
    target: website/content/en/docs/dev/internals.md
  - kind: code
    role: implementation
    target: pkg/store/instance.go#inspectStatusWithPIDFiles
  - kind: code
    role: implementation
    target: pkg/limatype/lima_yaml.go
---

# Instance

A Linux (or macOS, FreeBSD or Windows) virtual machine that Lima keeps for the User under its own name, with its configuration, its disk and the state of its host-side processes.

## Information kept

- **Name** — the instance's unique name; `default` is the instance commands act on when none is given
- **Hostname** — the guest's host name, `lima-<name>`, by which the host reaches it
- **Directory** — where the instance's configuration, disk, logs and sockets are kept under the Lima home directory
- **VM type** — the virtualization driver that runs it (QEMU, VZ, krunkit, WSL2 or HCS), chosen when it is created
- **Guest OS** — the guest operating system: Linux, macOS, FreeBSD or Windows
- **Architecture** — the guest CPU architecture, the host's by default
- **Images** — the operating system images it boots from, in order of preference, each with an optional digest, kernel and initrd
- **CPUs** — how many virtual CPUs the guest gets
- **Memory** — how much memory the guest gets
- **Disk size** — the size of the guest's main disk
- **Mounts** — the host directories shared with the guest, each read-only unless marked writable, with where they appear in the guest and how they are shared
- **Port forwards** — the ordered rules saying which guest ports or sockets are forwarded to which host addresses or sockets, which are ignored, and which are static
- **Networks** — the Lima networks or sockets the guest joins besides its default user-mode network
- **Additional disks** — the Lima disks the guest attaches by name, with how each is formatted and mounted
- **Host block devices** — the macOS disk devices of the host attached to a VZ guest
- **Guest user** — the guest user's name, home directory and login shell
- **Provisioning** — the scripts and files applied in the guest on boot
- **Probes** — the readiness scripts that must succeed before the instance counts as ready
- **Containerd** — whether system-wide and rootless containerd run in the guest
- **Environment** — the environment variables set in the guest
- **Parameters** — named values the template's scripts, probes and rules read
- **Copy to host** — the guest files copied to the host once the guest is ready
- **Message** — the text shown to the User once the instance is ready
- **Plain mode** — On when mounts, dynamic port forwarding, containerd and the guest agent are switched off
- **SSH local port** — the port on the host's localhost through which the guest's SSH is reached
- **Guest disk** — the contents of the guest's main disk, written as the guest runs
- **Forwarded ports** — the guest ports currently forwarded to the host
- **Protection** — On when the instance is protected against accidental removal and factory reset
- **Automatic startup** — whether the host starts the instance by itself: Off, at login or at boot, and whether it is kept running
- **Lima version** — the version of Lima that created the instance
- **Errors** — the configuration or process errors found when the instance was last inspected

## States

### Stopped

The virtual machine and its host agent are not running.

### Running

The virtual machine and its host agent are running. A running instance whose
guest agent does not answer is reported as degraded: file sharing and port
forwarding may not work.

### Broken

Lima finds the instance's processes or configuration inconsistent — for example
the host agent running without the virtual machine, a host agent that no
longer answers, or a configuration that cannot be read — and records why in
its Errors.
