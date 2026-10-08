---
references:
  - kind: code
    role: implementation
    target: pkg/hostagent/events/events.go#Event
  - kind: code
    role: implementation
    target: cmd/limactl/watch.go#watchAction
  - kind: code
    role: implementation
    target: pkg/hostagent/hostagent.go#emitEvent
---

# Instance event

Something that happened to a running instance, as its host agent reports it: a change of status, provisioning progress, a port forwarded or released, or an error. Lima keeps the events with the instance, and `limactl watch` shows them.

## Information kept

- **Time** — when it happened
- **Status** — whether the instance is running, degraded or exiting
- **Errors** — the errors reported with it
- **SSH local port** — the host port of the guest's SSH at that time
- **Provisioning progress** — the latest line of the guest's provisioning log and whether provisioning has finished
- **Port forward** — a guest address forwarded to or released from a host address, or a forward that failed
