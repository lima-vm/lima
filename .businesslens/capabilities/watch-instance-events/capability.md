---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/watch.go#watchAction
  - kind: code
    role: implementation
    target: pkg/hostagent/events/events.go
---

# Watch events from instances

Follow the events of instances as they happen — status changes, forwarded ports, provisioning progress and errors — for named instances or for every instance, including those created while watching.
