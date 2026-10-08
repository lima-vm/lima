---
appliesTo:
  - type: entity
    id: instance-event
  - type: capability
    id: start-instance
  - type: capability
    id: stop-instance
  - type: capability
    id: forward-guest-ports
  - type: capability
    id: stop-forwarding-guest-ports
references:
  - kind: code
    role: implementation
    target: pkg/hostagent/events/events.go#Event
  - kind: code
    role: implementation
    target: pkg/hostagent/hostagent.go#emitEvent
---

# An instance's events record what its host agent reports

While an instance runs, its host agent records an event for each change of
status, each step of provisioning progress, each port forwarded, released or
failed, and each error. Commands run in the guest and files changed there are
not recorded.
