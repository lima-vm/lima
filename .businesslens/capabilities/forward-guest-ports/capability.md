---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: pkg/hostagent/port.go#portForwardRules
  - kind: code
    role: implementation
    target: pkg/limayaml/defaults.go#FillPortForwardDefaults
  - kind: doc
    role: context
    target: website/content/en/docs/config/port.md
  - kind: code
    role: implementation
    target: pkg/hostagent/hostagent.go#processGuestAgentEvents
  - kind: code
    role: implementation
    target: pkg/guestagent/guestagent_linux.go#Events
  - kind: code
    role: implementation
    target: pkg/portfwd/forward.go#NewPortForwarder
  - kind: code
    role: implementation
    target: pkg/hostagent/port.go
---

# Forward guest ports

While an instance runs, forward each port a guest program listens on to the host, following the instance's port-forward rules and otherwise forwarding guest localhost ports to the same port on the host's localhost. TCP and UDP are forwarded through the guest agent; Unix sockets and static rules over SSH.
