---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/restart.go
  - kind: code
    role: implementation
    target: pkg/instance/restart.go
  - kind: code
    role: implementation
    target: cmd/limactl/restart.go#restartAction
  - kind: code
    role: implementation
    target: pkg/instance/restart.go#Restart
---

# Restart an instance

Stop a running instance and start it again, applying any configuration changes given with the command.
