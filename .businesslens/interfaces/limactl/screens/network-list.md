---
entities:
  - entity: network
    shows: [Name, Mode, Gateway, Interface]
entryPoints:
  - limactl: limactl network list
references:
  - kind: code
    role: implementation
    target: cmd/limactl/network.go#networkListAction
---

# Network list

Every Lima network, or those named.
