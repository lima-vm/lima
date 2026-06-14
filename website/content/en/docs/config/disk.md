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
limactl disk create NAME --size SIZE [--format qcow2|raw]
```

The supported formats are `qcow2` (default) and `raw`.

Example – create a 20 GiB disk named `data`:

```sh
limactl disk create data --size 20GiB
```

### Attaching a disk to an instance

{{< tabpane text=true >}}
{{% tab header="YAML" %}}
Add the disk name under `additionalDisks` before starting the instance:

```yaml
additionalDisks:
- name: data
  format: true     # format the disk on first use
  fsType: ext4     # filesystem to create when format is true
```
{{% /tab %}}
{{% tab header="CLI" %}}
Use `limactl edit` to attach while the instance is stopped:

```sh
limactl edit <instance> --set '.additionalDisks += [{"name":"data"}]'
```
{{% /tab %}}
{{< /tabpane >}}

### Resizing an additional disk

```sh
limactl disk resize NAME --size NEW-SIZE
```

### Deleting a disk

```sh
limactl disk delete NAME [NAME...]
```

> **Note:** A disk cannot be deleted while attached to a running instance. Stop the instance first or use `--force`.

---

## Resizing the VM's primary disk

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

## Attach host block devices

| ⚡ Requirement | Lima >= 2.3, macOS >= 14.0, vmType: vz |
| ------------- | -------------------------------------- |

Lima can attach a host block device directly to the guest.

Example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
# hdiutil attach -nomount ram://65536   # create a ramdisk to test without a USB key or real drive
limactl start --vm-type=vz --block-device=/dev/disk4 template:default
```

`/dev/rdiskN` also works:

```bash
# hdiutil attach -nomount ram://65536   # create a ramdisk to test without a USB key or real drive
limactl start --vm-type=vz --block-device=/dev/rdisk4 template:default
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "vz"
blockDevices:
- /dev/disk4
- /dev/rdisk5
```
{{% /tab %}}
{{< /tabpane >}}

The `--block-device` flag can be specified multiple times to attach multiple host block devices.

### Sudoers setup

Opening a host block device requires a privileged helper on macOS.
Generate and install the Lima sudoers file with the explicit block-device opt-in before using `--block-device`:

```sh
limactl sudoers --block-device=/dev/disk4 >etc_sudoers.d_lima
less etc_sudoers.d_lima
sudo install -o root etc_sudoers.d_lima /etc/sudoers.d/lima
rm etc_sudoers.d_lima
```

The block-device entry is scoped to the current user and is only generated when the `limactl`
executable is installed in a root-owned path that cannot be modified by regular users.
Homebrew-installed `limactl` does not satisfy this requirement; install a root-owned copy such as
`sudo make PREFIX=/opt/lima install` before generating the sudoers entry.
The helper only accepts real macOS disk device nodes such as `/dev/disk4` and `/dev/rdisk4s1`.

### Notes

- The configured path must be a macOS disk device path under `/dev`, e.g. `/dev/disk4` or `/dev/rdisk4s1`.
- The guest will see a corresponding virtio `/dev/disk/by-id/virtio-disk4` block device attached, you are responsible for partitioning, formatting, and mounting any filesystems on the block devices
- Avoid mounting filesystems on shared block devices on both host and guest at the same time, it will cause corruption as most filesystems are not designed to be shared by multiple live kernels at once.
