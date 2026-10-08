---
domain: networks
references:
  - kind: code
    role: implementation
    target: pkg/networks/config.go#LoadConfig
  - kind: code
    role: implementation
    target: cmd/limactl/network.go#networkListAction
  - kind: doc
    role: context
    target: website/content/en/docs/config/network/_index.md
  - kind: code
    role: implementation
    target: pkg/networks/config.go
  - kind: code
    role: implementation
    target: cmd/limactl/network.go
---

# Network

A Lima network that instances can join besides their default user-mode network, kept in `networks.yaml`. Lima ships `user-v2`, `shared`, `bridged` and `host` networks.

## Information kept

- **Name** — the network's unique name
- **Mode** — `user-v2`, `shared`, `bridged` or `host`
- **Gateway** — the gateway address and netmask, for every mode but bridged
- **Interface** — the host interface a bridged network uses
