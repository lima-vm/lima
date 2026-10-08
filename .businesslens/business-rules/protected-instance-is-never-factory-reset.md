---
appliesTo:
  - type: capability
    id: factory-reset-instance
    contexts:
      - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/factory-reset.go#factoryResetAction
---

# A protected instance is never factory reset

Factory resetting an instance is refused while it is protected, before
anything is stopped or erased.
