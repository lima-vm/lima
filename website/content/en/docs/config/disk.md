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

### Sudoers setup

Opening a host block device requires a privileged helper on macOS.
There are two setup steps: a protected helper installation, then authorization for
the specific devices. `start --block-device` does not grant root access automatically.

The helper and every ancestor directory must be root-owned and not writable by regular
users. If your installation already meets this requirement, skip to the sudoers commands below.
For a user-writable installation (including typical Homebrew installs), download the
macOS binary archive for your architecture from the [Lima releases](https://github.com/lima-vm/lima/releases)
and extract it into a protected prefix. A source build is not required:

```sh
# Replace VERSION and ARCH with the filename of the downloaded release archive.
sudo install -d -o root -g wheel -m 0755 /opt/lima
sudo tar --no-same-owner -xzf lima-VERSION-Darwin-ARCH.tar.gz -C /opt/lima
```

Use `/opt/lima/bin/limactl` directly for the commands below and for starting the VM
if you installed this copy. The helper is found relative to the invoked executable;
a symlink in another prefix does not select the protected installation.

Generate the grant as your regular user, inspect it, check its syntax, then install it.
Replace `/dev/disk4` with the intended device and include every grant for your user
that you want to retain:

```sh
limactl sudoers --block-device=/dev/disk4 >etc_sudoers.d_lima
less etc_sudoers.d_lima
visudo -cf etc_sudoers.d_lima
sudo install -o root -g wheel -m 0444 etc_sudoers.d_lima /etc/sudoers.d/lima
rm etc_sudoers.d_lima
```

The generated block-device entries are scoped to the current user. Regeneration cannot
reproduce another user's entries; preserve those lines manually in the temporary file and
validate the combined file with `visudo -cf` before installing it. Do not run `limactl sudoers`
as root.
The helper only accepts real macOS disk device nodes such as `/dev/disk4` and `/dev/rdisk4s1`.
Use builtin VZ through the installed `limactl` executable; external VZ drivers and
renamed executables are not supported for block-device attachment.

Include every current-user `--block-device` grant you want to keep when regenerating this file.
If network validation fails, the generated file omits network grants; installing it over
an existing file removes those grants. Review the output before replacing it.
With `--block-device`, `sudoers --check` compares the entire file with output generated for
the current user, so it reports a combined multi-user or manually preserved file as out of
sync. Without `--block-device`, it checks that the active network fragment is present and
allows additional entries. `visudo -cf` validates syntax, not whether grants are intended.
Device grants persist until removed from sudoers and allow root read-write access to the
device slot, not a particular physical disk. macOS may reuse `/dev/diskN` for another disk.
Never grant your boot/system disk; raw write access can compromise the host.
To revoke one of your device grants, stop the VM and regenerate the temporary file with all
of your remaining grants. Preserve any network grants and other users' entries manually,
validate the combined file with `visudo -cf`, then install it as above. When revoking your
last device grant without a valid network configuration, generation without `--block-device`
fails; instead, use `sudo visudo -f /etc/sudoers.d/lima` to remove only your block-device
rule and preserve the rest of the file. A manual edit may be syntactically valid but still
reported out of sync because `--check --block-device` compares the whole generated file
byte-for-byte, including formatting.
Do not remove the whole file because it may also authorize Lima networking or other users.

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

### Notes

- The configured path must be a macOS disk device path under `/dev`, e.g. `/dev/disk4` or `/dev/rdisk4s1`.
- The guest sees `/dev/disk/by-id/virtio-<device basename>`, with the basename capped at 20 ASCII bytes (e.g. `virtio-disk4` or `virtio-rdisk5`). You are responsible for partitioning, formatting, and mounting filesystems on the block devices.
- Never mount the same filesystem on the host and a guest, or in two guests, at the same time: this can corrupt data. Instances sharing a `LIMA_HOME` lock the whole disk, including its raw/block aliases and partitions, until the VM stops. This lock does not coordinate other users, other `LIMA_HOME` directories, or host mounts.
