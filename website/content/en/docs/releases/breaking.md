---
title: Breaking changes
weight: 20
---

## [v2.0.0](https://github.com/lima-vm/lima/releases/tag/v2.0.0)
- `/tmp/lima` is no longer mounted by default.
- SSH port is no longer hard-coded to 60022 for the "default" instance.
- Port forwarding with `sudo nerdctl run -p` no longer works with nerdctl prior to v2.1.6.
- The default of `guestIPMustBeZero` was changed from `false` to `true` when `guestIP` is `0.0.0.0`.

## [v1.1.0](https://github.com/lima-vm/lima/releases/tag/v1.1.0)
- The `lima-additional-guestagent` package was split from the main `lima` package.

## [v1.0.0](https://github.com/lima-vm/lima/releases/tag/v1.0.0)
- The default [VM type](../config/vmtype/_index.md) was changed from `qemu` to `vz` on macOS hosts with the support for `vz`.
- The default [mount type](../config/mount.md) was changed from `reverse-sshfs` to `virtiofs` for `vz`, `9p` for `qemu`.
- `socket_vmnet` binary has to be strictly owned by root.
- The default value of `ssh.loadDotSSHPubKeys` was changed from `true` to `false`.
- Several templates were removed or renamed.

## [v0.22.0](https://github.com/lima-vm/lima/releases/tag/v0.22.0)
- Support for `vde_vmnet` was dropped in favor of `socket_vmnet`.

## [v0.3.0](https://github.com/lima-vm/lima/releases/tag/v0.3.0)
- `limactl start` no longer starts a VM in the foreground.

See also <https://github.com/lima-vm/lima/releases>.