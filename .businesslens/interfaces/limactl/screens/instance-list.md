---
entities:
  - entity: instance
    shows: [Name, Hostname, SSH local port, VM type, Guest OS, Architecture, CPUs, Memory, Disk size, Directory, Protection, Automatic startup, Lima version, Networks, Additional disks, Parameters, Message, Errors]
entryPoints:
  - limactl: limactl list
references:
  - kind: code
    role: implementation
    target: cmd/limactl/list.go#listAction
  - kind: code
    role: implementation
    target: pkg/store/instance.go#PrintInstances
---

# Instance list

Every instance, or those named or matching a filter, with its status, SSH
address, VM type, resources and directory.
