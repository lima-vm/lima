---
title: Managed networks
weight: 34
---

These networks assign a "real" IP address that is reachable from the host.
The implementation depends on the host OS:

- [macOS]({{< ref "/docs/config/network/managed/macos" >}}): `socket_vmnet`
- [Linux]({{< ref "/docs/config/network/managed/linux" >}}): `lima-privileged-net`
