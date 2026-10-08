---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/stop.go#stopAction
  - kind: code
    role: implementation
    target: pkg/instance/stop.go#StopGracefully
  - kind: code
    role: implementation
    target: pkg/instance/stop.go#StopForcibly
  - kind: code
    role: implementation
    target: pkg/instance/stop.go
---

# Stop an instance

Shut running instances down gracefully, or force them to stop whatever their state. Stopping releases the instance's Lima disks.
