---
appliesTo:
  - type: entity
    id: instance
    facts: [Mounts]
references:
  - kind: code
    role: implementation
    target: pkg/limayaml/defaults.go
  - kind: doc
    role: context
    target: templates/_default/mounts.yaml
---

# Mounts are read-only unless marked writable

A host directory is shared with the guest read-only unless its mount is marked
writable. The default template shares the User's home directory read-only.
