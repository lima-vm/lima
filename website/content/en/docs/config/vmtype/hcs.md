---
title: HCS
weight: 5
---

> **Warning**
> "hcs" mode is experimental

| ⚡ Requirement | Lima >= 2.3 + (Windows >= Windows 11) |
| ----------------- | -------------------------------------------------------- |

"hcs" option makes use of native virtualization support provided by Windows' Host Compute System (HCS) API ([more info](https://learn.microsoft.com/en-us/virtualization/api/hcs/overview)).

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=hcs --plain --dns 8.8.8.8
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
# Example to run Ubuntu using vmType: hcs
vmType: hcs
images:
- location: "https://cloud-images.ubuntu.com/releases/resolute/release-20260720/ubuntu-26.04-server-cloudimg-amd64.img"
  arch: "x86_64"
  digest: "sha256:117816726abbdefc5ef3e38902e81a76f1c76c3610e709999d0885f9d5d9b477"
plain: true
dns:
- 8.8.8.8
```
{{% /tab %}}
{{< /tabpane >}}

### Caveats
- We tested it works only on Windows 11 (x86_64).
- Only plain mode is supported (no file mount, no dynamic port-forwarding).
- `qemu-img` is required for the image conversion; see [Disk images](#disk-images).
- The user must be a member of the Hyper-V Administrators group; see [Setup for HCS driver](#setup-for-hcs-driver)
- `limactl` should be run from a shell opened with Administrator privileges.
- Currently, only one instance can run using hcs at a time, beucause the HCN network subnet is fixed and cannot be shared by multiple instances.
- A DNS server has to be specified explicitly.


### Disk images
- The `hcs` driver can only boot from a `.vhdx` disk image.
- If the image is not a `.vhdx` (for example `.qcow2`), Lima automatically converts it to `.vhdx` by running `qemu-img convert -O vhdx`.
- Therefore, if you would like to download the image or to pass a disk image which is not `.vhdx`, `qemu-img` is required.

### Setup for HCS driver
1. **Enable virtualization on Windows:**
   Follow the instrunctions in [Microsoft guide](https://support.microsoft.com/en-us/windows/experience/enable-virtualization-on-windows) to enable virtualization on your Windows host.

2. **Add yourself to `Hyper-V Administrators` group:**
   Run the following command on powershell or cmd.exe with Run as administrator:
   ```bash
   net localgroup "Hyper-V Administrators" <user name> /add
   ```

   Then, restart the terminal or reboot the computer for the changes to take effect.

   To verify that your user account has been added to `Hyper-V Administrators` group, run:
   ```bash
   net localgroup "Hyper-V Administrators"
   ```
