---
title: Windows
weight: 2
---

| ⚡ Requirement | Lima >= 2.2, QEMU, swtpm |
|-------------------|-----------------------------|

Running Windows guests is experimentally supported since Lima v2.2.

{{< tabpane text=true >}}
{{% tab header="Windows 11" %}}
```
limactl start template:windows
```
{{% /tab %}}
{{% tab header="Windows server 2025" %}}
```
limactl start template:windows-2025
```
{{% /tab %}}
{{< /tabpane >}}

The user password is randomly generated and stored in the `%USERPROFILE%\password.txt` file in the VM.
Consider changing it after the first login.

By default, Windows 11 enables Trusted Platform Module (TPM) emulation because of the hardware requirement. However, you can turn it off (in that case, lima bypasses the hardware check). In order to use TPM emulation, you need to install `swtpm` on your host computer.

For Windows server 2025, TPM emulation is disabled by default. However, there are some benefits if you enable TPM emulation. For example, you can install [BitLocker disk encryption](https://learn.microsoft.com/en-us/windows/security/operating-system-security/data-protection/bitlocker/install-server) on your VM.

## Difference from Linux guests
- Several features are not implemented yet. See [Caveats](#caveats) below.

## Caveats
- For Windows 11 guest, you need to download the installer ISO manually from [here](https://www.microsoft.com/en-us/software-download/windows11)
- QEMU is the only VM driver that supports Windows guests
- Only plain mode is supported (no file mount, no dynamic port-forwarding)
- Booting Windows 11 may occasionally fail. If it fails, please delete the instance and try it again from scratch.
