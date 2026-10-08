---
appliesTo:
  - type: capability
    id: create-instance
  - type: capability
    id: clone-instance
  - type: capability
    id: rename-instance
references:
  - kind: code
    role: implementation
    target: pkg/limatype/dirnames/dirnames.go#ValidateInstName
  - kind: code
    role: implementation
    target: pkg/instance/create.go#Create
---

# An instance name is a short identifier

An instance name is a valid identifier, does not end in `.yaml` or `.yml`, and
is short enough for the instance's sockets to fit the host's socket path limit.
