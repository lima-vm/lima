---
title: Automatic Startup
weight: 4
---

| ⚡ Requirement | Lima >= 2.2 |
|----------------|-------------|

## Starting instances automatically

Use `limactl autostart enable` to register a Lima instance to start automatically.
Use `limactl autostart disable` to remove the registration.

### At user login (macOS and Linux)

```bash
# Register
limactl autostart enable default

# Unregister
limactl autostart disable default
```

On macOS this installs a LaunchAgent in `~/Library/LaunchAgents/`. On Linux it
installs a systemd user service. The instance starts in the background on the
next login and on subsequent logins.

### At system boot, without a user session (macOS only)

For headless macOS servers where no user session is expected, use
`--condition=boot`. This installs a system LaunchDaemon that starts the instance
at boot, before any user logs in.

```bash
# Register (prompts for sudo once)
limactl autostart enable --condition=boot k3s

# Unregister
limactl autostart disable k3s
```

The `--user` flag specifies which macOS user the instance runs as (default:
`$USER`). The plist is installed to
`/Library/LaunchDaemons/io.lima-vm.daemon.<instance>.plist`.
