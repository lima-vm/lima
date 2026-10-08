---
availability:
  - place: limactl
domain: networks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/network.go#networkListAction
  - kind: code
    role: implementation
    target: pkg/networks/config.go#LoadConfig
  - kind: code
    role: implementation
    target: cmd/limactl/network.go
  - kind: code
    role: implementation
    target: pkg/networks/config.go
---

# List networks

Show every Lima network, or those named.
