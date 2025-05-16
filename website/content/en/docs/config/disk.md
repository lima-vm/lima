---
title: Resize Disk Size
---

This guide explains how to increase the disk size of a Lima VM running on macOS when you've run out of space, as well as how to edit the disk size using the `limactl` CLI.

## Prerequisites
1. Sufficient free disk space on your host.
2. VM type considerations:
   - For `qemu` VM type: All methods below are supported
   - For `vz` VM type: Option 1 is supported

## Option 1: Resize Disk Using limactl

Lima supports editing the disk size of an existing instance using the `--disk` flag with the `limactl edit` command.  
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

## Option 2: Manually Resize Lima VM Disk

If you need to manually resize the disk, follow these steps:

1. **Stop the Lima VM** 

   ```sh
   limactl stop <vm-name>
   ```

2. **Locate your disk files**  
   Lima typically uses two disk files:
   - `basedisk`: The read-only base image
   - `diffdisk`: The writable disk that stores your changes (this is the one you'll resize)

   These files are usually located in `~/.lima/<vm-name>/` directory.

3. **Resize the diffdisk file using qemu-img**
    ```sh
    qemu-img resize ~/.lima/<vm-name>/diffdisk <new-size>
    ```

    Example for 200GB:
    ```sh
    qemu-img resize ~/.lima/<vm-name>/diffdisk 200G
    ```

   > Note: You may see a warning about the raw format not being specified. This is normal and can be avoided by explicitly specifying the format:  
   > `qemu-img resize -f raw ~/.lima/<vm-name>/diffdisk 200G`

4. **Start the Lima VM**  

   ```sh
   limactl start <vm-name>
   ```
   The filesystem will be automatically resized by `systemd-growfs` on boot. You will see something like:

   ```sh
   systemd-growfs[441]: Successfully resized "/" to 199.8G bytes
   ```

> **Note**: You can verify the new size via `df -h /`.
