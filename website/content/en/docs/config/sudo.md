---
title: Sudo
weight: 40
---

| ⚡ Requirement | Lima >= 2.2 |
|---------------|-------------|

By default, the guest user has passwordless sudo (`NOPASSWD:ALL`).

The password is only generated once, on first boot. If you manually change the
password and remove `~/password`, it will **not** be regenerated on subsequent
boots, the file is purely informational for the initial login.

```yaml
user:
  passwordlessSudo: false
```

## Caveats

- Requires `plain: true` on non-macOS guests ([GHSA-2j9v-p4xj-cjw2](https://github.com/lima-vm/lima/security/advisories/GHSA-2j9v-p4xj-cjw2)): the guest agent's Unix socket tunneling is disabled until it can safely refuse privileged socket tunneling with passwordless sudo off.
- Only applies to cloud-init provisioned guests (QEMU/vz). Setting `user.passwordlessSudo: false` on WSL2 (Windows) guests returns a validation error, as WSL2 does not support this configuration as of now.
- The guest user is already a member of the `adm` and `systemd-journal` groups, which grants passive read access to `/var/log/*` and the full system journal independent of sudo.
- `param` still relies on sudo internally (`param.env` is read via `sudo cat` inside the guest), so it may not work as expected when `user.passwordlessSudo: false` is set. A warning is printed in this case.
- **macOS**: not configurable. Password-required sudo is always on (except `/sbin/shutdown -h now`), matching the built-in [macOS guest](/docs/usage/guests/macos/) behavior. Setting `user.passwordlessSudo: true` on a macOS guest returns a validation error.
