---
title: Windows
weight: 2
---

| ⚡ Requirement | Lima >= 2.2, QEMU |
|-------------------|-----------------------------|

Running Windows guests is experimentally supported since Lima v2.2.

```bash
limactl start template:windows-2025
```

The user password is randomly generated and stored in the `%USERPROFILE%\password.txt` file in the VM.
Consider changing it after the first login.

For Windows server 2025, Trusted Platform Module (TPM) is not required. However, there are some benefits if your VM has TPM. For example, you can install [BitLocker disk encryption](https://learn.microsoft.com/en-us/windows/security/operating-system-security/data-protection/bitlocker/install-server). In order to use TPM emulation on your VM, you need to install `swtpm` and set `tpm: true` on your yaml file.

## Difference from Linux guests
- Several features are not implemented yet. See [Caveats](#caveats) below.

## Caveats
- Currently only Windows server 2025 (x86-64) is supported
- QEMU is the only VM driver that supports Windows guests
- Only plain mode is supported (no file mount, no dynamic port-forwarding)
