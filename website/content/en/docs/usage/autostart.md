---
title: Automatic Startup
weight: 4
---

| ⚡ Requirement | Lima >= 2.2 |
|----------------|-------------|

Lima instances can be registered to start automatically using `limactl autostart`.
Two conditions are supported: `login` (start when the user logs in) and `boot`
(start at system boot, before any user session). This replaces the older
`limactl start-at-login` command, which is deprecated as of Lima v2.2.

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

## Keep-alive behavior

By default (`--keep-alive=true`), launchd will automatically restart the Lima
host agent if it exits unexpectedly. To disable this:

```bash
limactl autostart enable --keep-alive=false default
```

This applies to both `--condition=login` (macOS LaunchAgent) and
`--condition=boot` (macOS LaunchDaemon). On Linux, restart behavior is
configured separately in the systemd unit file.

## Lima < 2.2

Use `limactl start-at-login` (equivalent to `limactl autostart enable --condition=login`):

```bash
# Register
limactl start-at-login default

# Unregister
limactl start-at-login --enabled=false default
```
