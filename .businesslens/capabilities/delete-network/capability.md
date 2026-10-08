---
availability:
  - place: limactl
domain: networks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/network.go#networkDeleteAction
  - kind: code
    role: implementation
    target: cmd/limactl/network.go
  - kind: code
    role: implementation
    target: pkg/networks/config.go
---

# Delete a network

Remove Lima networks from `networks.yaml`. Lima does not check whether instances use them, so deleting always requires `--force`.
