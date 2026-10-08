---
availability:
  - place: limactl
domain: automatic-startup
references:
  - kind: code
    role: implementation
    target: cmd/limactl/autostart.go
  - kind: code
    role: implementation
    target: cmd/limactl/autostart_darwin.go
  - kind: code
    role: implementation
    target: cmd/limactl/autostart_others.go
  - kind: code
    role: implementation
    target: cmd/limactl/autostart_darwin.go#autostartDisableAction
  - kind: code
    role: implementation
    target: pkg/autostart/autostart.go#UnregisterFromStartAtLogin
---

# Disable automatic startup

Unregister an instance from automatic startup.
