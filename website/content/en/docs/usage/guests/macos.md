---
title: macOS
weight: 2
---

| ⚡ Requirement | Lima >= 2.1, macOS, ARM  |
|-------------------|-----------------------------|

Running macOS guests is experimentally supported since Lima v2.1.

{{< tabpane text=true >}}
{{% tab header="macOS only" %}}
```bash
limactl start template:macos
```
{{% /tab %}}
{{% tab header="With Homebrew" %}}
```bash
limactl start template:homebrew-macos
```
{{% /tab %}}
{{< /tabpane >}}

The user password is randomly generated and stored in the `~/password` file in the VM.
Consider changing it after the first login.

```bash
limactl shell macos cat /Users/${USER}.guest/password
```

## Difference from Linux guests
- Password login is enabled
- Password-less sudo is disabled, except for `/sbin/shutdown -h now`
- Several features are not implemented yet. See [Caveats](#caveats) below.

## Caveats
- No support for turning off the video display.
- No support for automatic port forwarding.
  Use `ssh -L` to manually set up port forwarding, or,
  use the [`vzNAT`](../../config/network/vmnet.md#vznat) network to access the guest by its IP.
- No support for installing custom `caCerts`
