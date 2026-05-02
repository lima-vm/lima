---
title: Disks
---

This guide explains how to manage Lima disks: standalone raw/qcow2 block devices that persist independently of any instance.

## Additional Disks (limactl disk)

Lima disks can be shared across instances and survive instance deletion.

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

**Via YAML:** add the disk name under `additionalDisks` before starting the instance:

```yaml
additionalDisks:
  - name: data
    format: true   # format the disk on first use
```

**Via CLI:** use `limactl edit` to attach while the instance is stopped:

```sh
limactl edit <instance> --set '.additionalDisks += [{"name":"data"}]'
```

### Resizing a disk

```sh
limactl disk resize NAME --size NEW-SIZE
```

### Deleting a disk

```sh
limactl disk delete NAME [NAME...]
```

> **Note:** A disk cannot be deleted while attached to a running instance. Stop the instance first or use `--force`.

---

## Resize VM Disk Using limactl

Starting with v1.1, Lima supports editing the disk size of an existing instance using the `--disk` flag with the `limactl edit` command.
This is the recommended and simplest way to resize your VM disk.

```sh
limactl edit <vm-name> --disk <new-size>
```

Example for 20GB:

```sh
limactl edit default --disk 20
```

> **Note:**
> - Increasing disk size is supported, but shrinking disks is not recommended.
> - The instance may need to be stopped before editing disk size.
