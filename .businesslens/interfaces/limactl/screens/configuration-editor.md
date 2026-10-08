---
entities:
  - entity: instance
    shows: [Name, VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]
    collects: [VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]
  - entity: template
    shows: [Name, Base templates, Settings, Parameters]
    collects: [Base templates, Settings, Parameters]
references:
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#chooseNextCreatorState
  - kind: code
    role: implementation
    target: cmd/limactl/edit.go#editAction
  - kind: code
    role: implementation
    target: pkg/editutil/editutil.go
---

# Configuration editor

An instance's or a template's configuration opened in the User's text editor:
before an instance is created from a template, when an instance is edited, or
when a template file is edited. Before creating, the User may proceed with the
configuration as it is, open it in the editor, choose another template or exit.
