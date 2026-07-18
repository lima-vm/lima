---
title: Configuration guide
weight: 5
---

For all the configuration items, see <https://github.com/lima-vm/lima/blob/master/templates/default.yaml>.

The current default spec:
- OS: Ubuntu
- CPU: 4 cores
- Memory: 4 GiB
- Disk: 100 GiB
- Mounts: `~` (read-only), `/tmp/lima` (writable; removed in Lima v2.0)
- SSH: 127.0.0.1:<Random port>
