---
title: Sudo
weight: X
---

By default, the guest user has passwordless sudo (`NOPASSWD:ALL`).

Set `user.passwordlessSudo: false` to require a password for `sudo` inside the guest.
When disabled, a random password is generated on first boot and saved to `~/password`
(mode `0600`) inside the guest. Rotate it manually after first login.

```yaml
user:
  passwordlessSudo: false
```

## Constraints

- Requires `plain: true` ([GHSA-2j9v-p4xj-cjw2](https://github.com/lima-vm/lima/security/advisories/GHSA-2j9v-p4xj-cjw2)):
  the guest agent's Unix socket tunneling is disabled until it can safely refuse
  privileged socket tunneling with passwordless sudo off.
- Only applies to cloud-init provisioned guests (QEMU/vz). WSL2 instances ignore this setting.
- **macOS**: not configurable. Password-required sudo is always on (except `/sbin/shutdown -h now`),
  matching the built-in [macOS guest](/docs/usage/guests/macos/) behavior. Setting
  `passwordlessSudo: true` on a macOS guest returns a validation error.
