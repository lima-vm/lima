---
title: Disks
---

This guide explains how to increase the disk size of a Lima VM running on macOS when you've run out of space, as well as how to edit the disk size using the `limactl` CLI.

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
