---
title: Network
weight: 30
---

See the following flowchart to choose the best network for you:
```mermaid
flowchart
connect_to_vm_via{"Connect to the VM via"} -- "localhost" --> default["Default"]
  connect_to_vm_via -- "IP" --> connect_from{"Connect to the VM IP from"}
  connect_from -- "Host" --> vm{"VM type"}
  vm -- "vz" --> vzNAT["vzNAT (see the VMNet page)"]
  vm -- "qemu" --> shared["socket_vmnet (shared)"]
  connect_from -- "Other VMs" --> userV2["user-v2"]
  connect_from -- "Other hosts" --> bridged["socket_vmnet (bridged)"]
```

## Managing named networks (limactl network)

Lima networks defined in `~/.lima/_config/networks.yaml` provide named
interfaces that can be shared across instances.

### Listing networks

```sh
limactl network list
# or the short alias:
limactl network ls
# machine-readable output:
limactl network list --json
```

### Creating a network

```sh
limactl network create NAME --gateway CIDR
```

Example:

```sh
limactl network create mynet --gateway 192.168.42.1/24
```

### Attaching a network to an instance

Add the network name under the `networks` key before starting the instance:

```yaml
networks:
  - lima: mynet
```

### Deleting a network

```sh
limactl network delete --force NAME [NAME...]
```

> **Note:** `--force` is currently required.
