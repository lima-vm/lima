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

| ⚡ Requirement | Lima >= 2.2 |
| ------------- | ----------- |

Lima can attach a host block device directly to the guest.

Supported backends:

| Host        | vmType            | Requirement                  |
| ----------- | ----------------- | ---------------------------- |
| macOS       | `vz`              | macOS >= 14.0                |
| macOS       | `qemu`, `krunkit` |                              |
| Linux, BSDs | `qemu`            |                              |
| Windows     | `qemu`            | elevated (Administrator) run |

WSL2 is not supported, because `wsl --mount` attaches disks globally to all
distros rather than to a single instance; use `vmType: qemu` on Windows
instead.

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

### Device access

No setup is required. On Unix hosts (macOS, Linux, BSDs) Lima resolves device
access in order:

1. **Direct open**: when the current user already has read-write access to
   the device node (e.g. user-created ramdisks on macOS, or membership in the
   `disk` group on Linux), the device is opened directly and no privilege
   escalation happens at all.
2. **Cached or passwordless sudo**: when a NOPASSWD sudoers rule (see below)
   or a still-valid cached sudo timestamp exists, the privileged helper runs
   without prompting.
3. **Password prompt**: otherwise `limactl start` asks for the sudo password
   once on the terminal before booting the VM.

The privileged helper (`limactl sudo-open-block-device`) only ever touches
the requested device node, and the VM itself never runs as root. With `vz`
and with `qemu` on Linux/BSD hosts, the helper opens the device as root and
passes the file descriptor back to the unprivileged Lima process. With `qemu`
and `krunkit` on macOS the descriptor handoff is impossible (opening
`/dev/fd/N` re-checks the device permissions, and QEMU's fd sets are broken
by an `fcntl` quirk for disk descriptors), so the helper instead changes the
ownership of the device node to the current user — the same thing an
administrator would do by hand with `sudo chown` — and the backend opens it
directly. devfs ownership reverts when the device is detached or the host
reboots.

To avoid the password prompt entirely (e.g. for headless or autostarted
instances), install the Lima sudoers file once:

```sh
limactl sudoers >etc_sudoers.d_lima
less etc_sudoers.d_lima
sudo install -o root etc_sudoers.d_lima /etc/sudoers.d/lima
rm etc_sudoers.d_lima
```

On macOS `limactl sudoers` emits the entries for both the vmnet helpers and
the block-device helper; on other Unix hosts it emits only the block-device
helper entry, scoped to the current user.

On Linux, adding the user to the group owning the device node also avoids
any prompting, via the direct-open path:

```sh
sudo usermod -aG disk "$USER"
```

On Windows there is no helper: QEMU opens the raw device path directly, so
`limactl start` must run from an elevated (Administrator) prompt, and the
disk should be taken offline first (e.g. `diskpart` > `select disk 2` >
`offline disk`) so Windows and the guest do not use it at the same time.

### Notes

- On Unix hosts the configured path must be an absolute host path under `/dev`, e.g. `/dev/disk4`, `/dev/rdisk4`, `/dev/sdc`, etc. On Windows it is a raw device path such as `\\.\PhysicalDrive2` (use the equivalent `//./PhysicalDrive2` form in MSYS2/Git-Bash shells, which mangle leading backslashes in arguments).
- With `vz` and `qemu`, the guest will see a corresponding virtio `/dev/disk/by-id/virtio-disk4` block device attached. krunkit does not support setting the device identifier, so the device only shows up as a plain `/dev/vdX` device. You are responsible for partitioning, formatting, and mounting any filesystems on the block devices.
- Avoid mounting filesystems on shared block devices on both host and guest at the same time, it will cause corruption as most filesystems are not designed to be shared by multiple live kernels at once.
