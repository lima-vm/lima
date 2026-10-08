---
availability:
  - place: limactl
domain: networks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/network.go#networkCreateAction
  - kind: code
    role: implementation
    target: cmd/limactl/network.go
  - kind: code
    role: implementation
    target: pkg/networks/config.go
---

# Create a network

Add a Lima network to `networks.yaml`, `user-v2` unless another mode is chosen.
