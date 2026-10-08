---
appliesTo:
  - type: entity
    id: instance
    facts: [VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]
  - type: entity
    id: template
    facts: [Base templates, Settings, Parameters]
references:
  - kind: code
    role: implementation
    target: pkg/limayaml/validate.go
  - kind: code
    role: implementation
    target: cmd/limactl/edit.go#saveRejectedYAML
---

# An instance configuration is saved only when it validates

Creating, editing, cloning and renaming with edit flags validate the
configuration, including what the chosen VM type supports, before saving it. A
configuration that does not validate replaces nothing, and when it came from
the User's editor or flags it is kept as `lima.REJECTED.yaml` in the current
directory.
