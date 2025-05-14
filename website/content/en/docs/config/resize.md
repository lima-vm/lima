---
title: Resize Disk Size on macOS
weight: 11
---

This guide explains how to increase the disk size of a Lima VM running on macOS when you've run out of space.

## Prerequisites
1. Sufficient free disk space on your host.
2. Using `Qemu` VM type for the lima.

## Steps to Resize Lima VM Disk

1. Stop the Lima VM - 
```sh
limactl stop <vm-name>
```

2. Locate your disk files, lima typically uses two disk files:
- `basedisk`: The read-only base image
- `diffdisk`: The writable disk that stores your changes (this is the one you'll resize)

These files are usually located in `~/.lima/<vm-name>/` directory.

3. Resize the diffdisk file using qemu-img
```sh
qemu-img resize ~/.lima/<vm-name>/diffdisk <new-size>
```
Example for 200GB:
```
qemu-img resize ~/.lima/<vm-name>/diffdisk 200G
```

> Note: You may see a warning about the raw format not being specified. This is normal and can be avoided by explicitly specifying the format:
> `qemu-img resize -f raw ~/.lima/<vm-name>/diffdisk 200G`


4. Start the Lima VM -
```
limactl start <vm-name>
```
The filesystem will be automatically resized by  `systemd-growfs` on boot, you will see something like below:

```sh
systemd-growfs[441]: Successfully resized "/" to 199.8G bytes
```

> Note: You can verify the new size via `df -h /`.
