---
appliesTo:
  - type: capability
    id: forward-guest-ports
    contexts:
      - place: limactl
references:
  - kind: code
    role: implementation
    target: pkg/hostagent/port.go#portForwardRules
  - kind: code
    role: implementation
    target: pkg/limayaml/defaults.go#FillPortForwardDefaults
---

# Guest localhost ports are forwarded to the host's localhost

Unless a port-forward rule says otherwise, a port a guest program listens on at
localhost (or on every address) is forwarded to the same port on the host's
localhost. The rules are tried in order and the first match wins.
