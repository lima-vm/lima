---
appliesTo:
  - type: entity
    id: instance
    facts: [CPUs, Memory, Disk size]
references:
  - kind: code
    role: implementation
    target: pkg/limayaml/defaults.go#defaultCPUs
  - kind: code
    role: implementation
    target: pkg/limayaml/defaults.go#defaultMemory
  - kind: code
    role: implementation
    target: pkg/limayaml/defaults.go#defaultDiskSizeAsString
---

# Default guest resources are bounded by the host

Unless configured, an instance gets four CPUs or the host's count if smaller,
4 GiB of memory or half the host's if smaller, and a 100 GiB disk. WSL2
instances do not configure them.
