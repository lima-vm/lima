---
title: Snapshots
weight: 5
---

| ⚡ Requirement | Lima >= v0.16.0, QEMU or krunkit |
|----------------|---------------------------------|

A snapshot captures the complete state of an instance's disk at a specific point in time. This allows you to quickly restore the virtual machine to a known, working state if a risky operation (such as an OS upgrade, configuration test, etc.) fails.

{{% alert title="Warning" color="danger" %}}
The **limactl snapshot** feature is currently **experimental** and may change in future releases. The instance **must be stopped** before creating, applying, or deleting a snapshot.
{{% /alert %}}

## Creating a snapshot

To create a snapshot, ensure the instance is stopped and provide a unique tag name for it:

```bash
limactl stop default
limactl snapshot create default --tag before-upgrade
```

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

## Deleting a snapshot

To remove a snapshot and free up disk space:

```bash
limactl snapshot delete default --tag before-upgrade
```
