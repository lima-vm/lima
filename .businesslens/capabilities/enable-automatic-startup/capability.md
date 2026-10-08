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
    target: cmd/limactl/start-at-login_unix.go
  - kind: doc
    role: context
    target: website/content/en/docs/usage/autostart.md
  - kind: code
    role: implementation
    target: cmd/limactl/autostart_darwin.go#autostartEnableAction
  - kind: code
    role: implementation
    target: cmd/limactl/autostart_others.go#autostartEnableAction
  - kind: code
    role: implementation
    target: pkg/autostart/autostart.go#RegisterToStartAtLogin
---

# Enable automatic startup

Register an instance with the host's service manager to start by itself — at login on macOS and Linux, or at boot on macOS — and to be started again if its host agent exits unexpectedly. The deprecated `limactl start-at-login` does the same for startup at login.
