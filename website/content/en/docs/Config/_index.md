---
title: Configuration guide
weight: 5
---

For all the configuration items, see <https://github.com/lima-vm/lima/blob/master/examples/default.yaml>.

The current default spec:
- OS: Ubuntu
- CPU: 4 cores
- Memory: 4 GiB
- Disk: 100 GiB
- Mounts: `~` (read-only), `/tmp/lima` (writable)
- SSH: 127.0.0.1:60022

{{% alert title="Temp" color="info" %}}
Use `$TMPDIR` to change the location of `/tmp` on Unix.
Use `$TEMP` to change the location of `/tmp` on Windows.
{{% /alert %}}
