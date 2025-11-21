---
title: Deprecated features
weight: 10
---

The following features are deprecated:

- `limactl show-ssh` command: deprecated in v0.18.0 (Use `ssh -F ~/.lima/default/ssh.config lima-default` instead)
- Ansible provisioning mode: deprecated in Lima v1.1.0 (Use `ansible-playbook playbook.yaml` after the start instead)
- `limactl --yes` flag: deprecated in Lima v2.0.0 (Use `limactl (clone|rename|edit|shell) --start` instead)
- Environment variable `LIMA_SSH_OVER_VSOCK`: deprecated in Lima v2.0.2 (Use the YAML property `.ssh.overVsock`)

## Removed features
- YAML property `network`: deprecated in [Lima v0.7.0](https://github.com/lima-vm/lima/commit/07e68230e70b21108d2db3ca5e0efd0e43842fbd)
  and removed in Lima v0.14.0, in favor of `networks`
- YAML property `useHostResolver`: deprecated in [Lima v0.8.1](https://github.com/lima-vm/lima/commit/eaeee31b0496174363c55da732c855ae21e9ad97)
  and removed in Lima v0.14.0,in favor of `hostResolver.enabled`
- VDE support, including VNL and `vde_vmnet`: deprecated in [Lima v0.12.0](https://github.com/lima-vm/lima/pull/851/commits/b5e0d5abd0fb2f74b7ddf8faea7a855b5a14ceda)
  and removed in Lima v0.22.0, in favor of `socket_vmnet`
- CentOS 7 guest support: deprecated in Lima v0.9.2 and removed in Lima v0.23.0, in favor of CentOS Stream, AlmaLinux, and Rocky Linux.

## Undeprecated features
- Loading non-strict YAMLs (i.e., YAMLs with unknown properties): once deprecated in Lima v0.12.0, but undeprecated in Lima v1.0.4
