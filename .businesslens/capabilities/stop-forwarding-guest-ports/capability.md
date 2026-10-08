---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: pkg/hostagent/hostagent.go#processGuestAgentEvents
  - kind: code
    role: implementation
    target: pkg/hostagent/port.go
---

# Stop forwarding guest ports

When a guest program stops listening on a forwarded port, stop forwarding the host port to it.
