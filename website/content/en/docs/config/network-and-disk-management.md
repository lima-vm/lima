---
title: Network and Disk management
---

Lima ships with `limactl` subcommands for managing **networks** (Lima-specific
virtual networks that can be shared across instances) and **additional disks**
(extra block devices that survive instance deletion).

## Networks

Lima networks are defined in `~/.lima/_config/networks.yaml` and provide
named network interfaces that can be attached to one or more instances.

### Listing networks

```sh
limactl network list
# or the short alias:
limactl network ls
```

Use `--json` to get machine-readable output:

```sh
limactl network list --json
```

### Creating a network

```sh
limactl network create NAME --gateway CIDR
```

Example – create a network with gateway `192.168.42.1/24`:

```sh
limactl network create mynet --gateway 192.168.42.1/24
```

### Deleting a network

```sh
limactl network delete --force NAME [NAME...]
```

> **Note:** `--force` is currently required.

### Attaching a network to an instance

Add the network name to the instance's YAML configuration under the `networks`
key before starting the instance:

```yaml
networks:
  - lima: mynet
```

See the [network configuration](./network/) pages for details on `user-v2`,
`vmnet`, and other network modes.

---

## Disks

Lima disks are standalone raw/qcow2 block devices that exist independently of
any instance. You can attach them to an instance so that data persists even
when the instance is deleted and recreated.

### Listing disks

```sh
limactl disk list
# or the short alias:
limactl disk ls
```

### Creating a disk

```sh
limactl disk create NAME --size SIZE [--format qcow2]
```

Example – create a 20 GiB disk named `data`:

```sh
limactl disk create data --size 20GiB
```

### Attaching a disk to an instance

Add the disk name to the instance's YAML configuration under the `additionalDisks`
key before starting the instance:

```yaml
additionalDisks:
  - name: data
    format: true   # format the disk on first use
```

### Resizing a disk

```sh
limactl disk resize NAME --size NEW-SIZE
```

### Deleting a disk

```sh
limactl disk delete NAME [NAME...]
```

> **Note:** You cannot delete a disk that is currently attached to a running
> instance. Stop the instance first or use `--force`.

---

For further detail on each command, run `limactl network --help` or
`limactl disk --help`.
