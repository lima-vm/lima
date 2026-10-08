---
appliesTo:
  - type: entity
    id: instance
    facts: [Forwarded ports]
references:
  - kind: code
    role: implementation
    target: pkg/hostagent/port.go#portForwardRules
---

# The SSH ports are never forwarded automatically

The guest's SSH port and the instance's SSH local port are excluded from
forwarding before any of the instance's own port-forward rules are tried.
