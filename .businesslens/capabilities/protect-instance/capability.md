---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/protect.go#protectAction
  - kind: code
    role: implementation
    target: pkg/limatype/lima_instance.go#Protect
---

# Protect an instance

Protect instances against accidental removal by `limactl delete` and `limactl factory-reset`.
