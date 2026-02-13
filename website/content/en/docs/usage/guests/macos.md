---
title: macOS
weight: 2
---

| ⚡ Requirement | Lima >= 2.1, macOS, ARM  |
|-------------------|-----------------------------|

Running macOS guests is experimentally supported since Lima v2.1.

```bash
limactl start template:macos
```

A password prompt is shown during creating an instance, so as to run
the `chown root:wheel ~/.lima/_mnt/0/...` command on the host.
This password is not used for setting up the user account in the VM.

The user password is randomly generated and stored in the `~/password` file in the VM.
Consider changing it after the first login.

```bash
limactl shell macos cat /Users/${USER}.guest/password
```

## Difference from Linux guests
- Password login is enabled
- Password-less sudo is disabled
- Several features are not implemented yet. See [Caveats](#caveats) below.

## Caveats
- No support for graceful `limactl stop`.
  Shutdown the VM from the GUI, or use `limactl stop -f` with caution.
- No support for turning off the video display.
- No support for mounting host directories.
  Use `limactl cp` or `limactl shell --sync` to share files with the host.
- No support for automatic port forwarding.
  Use `ssh -L` to manually set up port forwarding, or,
  use the [`vzNAT`](../../config/network/vmnet.md#vznat) network to access the guest by its IP.
- No support for installing custom `caCerts`
