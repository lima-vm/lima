---
title: Deprecated features
weight: 10
---

The following features are deprecated:

- VDE support, including VNL and `vde_vmnet`
- CentOS 7 support
- Loading non-strict YAMLs (i.e., YAMLs with unknown properties)
- `limactl show-ssh` command (Use `ssh -F ~/.lima/default/ssh.config lima-default` instead)

## Removed features
- YAML property `network`: deprecated in [Lima v0.7.0](https://github.com/lima-vm/lima/commit/07e68230e70b21108d2db3ca5e0efd0e43842fbd)
  and removed in Lima v0.14.0, in favor of `networks`
- YAML property `useHostResolver`: deprecated in [Lima v0.8.1](https://github.com/lima-vm/lima/commit/eaeee31b0496174363c55da732c855ae21e9ad97)
  and removed in Lima v0.14.0,in favor of `hostResolver.enabled`
