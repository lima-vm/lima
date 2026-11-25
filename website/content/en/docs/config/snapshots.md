---
title: Snapshots
weight: 5
---

| ⚡ Requirement | Lima >= v0.16.0 (QEMU), Lima >= v2.3.0 (krunkit), Lima >= v2.4.0 (VZ) |
|----------------|-----------------------------------------------------------------------|

A snapshot captures the complete state of an instance's disk at a specific point in time. This allows you to quickly restore the virtual machine to a known, working state if a risky operation (such as an OS upgrade, configuration test, etc.) fails.

With VZ and krunkit, a snapshot is an APFS clone of the instance's boot disk, so the instance directory has to be on an APFS volume.
With VZ, the snapshot also includes the EFI variables, and the auxiliary storage of macOS guests.
Additional disks are not included.

{{% alert title="Warning" color="danger" %}}
The **limactl snapshot** feature is currently **experimental** and may change in future releases. The instance **must be stopped** before creating, applying, or deleting a snapshot,
except for a QEMU instance without mounts, which can also be running.
{{% /alert %}}

## Creating a snapshot

To create a snapshot, ensure the instance is stopped and provide a unique tag name for it:

```bash
limactl stop default
limactl snapshot create default --tag before-upgrade
```

A QEMU instance without mounts can also be running. The snapshot then also captures the memory
and device state of the instance.

## Listing snapshots

To view the existing snapshots and their tags, use the `list` command:

```bash
limactl snapshot list default
```

The output will display a table with the snapshot ID, its tag, and the creation date.

## Applying a snapshot

To restore the instance to a previously saved state, use the `apply` command with the tag:

```bash
limactl stop default
limactl snapshot apply default --tag before-upgrade
```

A running QEMU instance without mounts accepts only a snapshot that was created while it was running.
A snapshot created on a stopped instance contains only the disk, and has to be applied while the instance is stopped.
After applying a snapshot to a running instance, `limactl snapshot apply` resets the persistent SSH connection,
so `limactl shell` keeps working, but SSH port forwards are not restored. Restart the instance to bring them back.

## Deleting a snapshot

To remove a snapshot and free up disk space:

```bash
limactl snapshot delete default --tag before-upgrade
```
