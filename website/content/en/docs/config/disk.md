---
title: Disks
---

This guide explains how to resize a Lima VM disk and how to attach host block devices.

## Resize Disk Using limactl

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

| ⚡ Requirement | Lima >= 2.2, macOS >= 14.0, vmType: vz |
| ------------- | --------------------------------------- |

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
Generate and install the Lima sudoers file before using `--block-device`:

```sh
limactl sudoers >etc_sudoers.d_lima
less etc_sudoers.d_lima
sudo install -o root etc_sudoers.d_lima /etc/sudoers.d/lima
rm etc_sudoers.d_lima
```

`limactl sudoers` emits the entries needed for both vmnet helpers and block-device helpers.

### Notes

- The configured path must be an absolute host path under `/dev`, e.g. `/dev/disk4`, `/dev/rdisk4`, `/dev/sdc`, etc.
- The guest will see a corresponding virtio `/dev/disk/by-id/virtio-disk4` block device attached, you are responsible for partitioning, formatting, and mounting any filesystems on the block devices
- Avoid mounting filesystems on shared block devices on both host and guest at the same time, it will cause corruption as most filesystems are not designed to be shared by multiple live kernels at once.
