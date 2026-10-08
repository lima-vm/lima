---
appliesTo:
  - type: entity
    id: instance
    facts: [VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]
references:
  - kind: code
    role: implementation
    target: pkg/limayaml/defaults.go#FillDefault
  - kind: doc
    role: context
    target: templates/default.yaml
---

# Global overrides take precedence over instance settings and defaults

Each time Lima loads an instance's configuration, it completes what the
instance leaves unset from `$LIMA_HOME/_config/default.yaml` and Lima's built-in
defaults, and `$LIMA_HOME/_config/override.yaml` overrides it; a change to these
files therefore reaches existing instances. Lists start with the overrides, then the instance's own entries, then the
defaults, so an override's port-forward rule is tried first; mounts of the same
host directory are merged, the highest-priority writable setting winning.
