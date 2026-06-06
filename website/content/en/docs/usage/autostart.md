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
`--condition=boot` (macOS LaunchDaemon). On Linux, the flag sets the systemd unit's
`Restart=` directive: `on-failure` when enabled (the default), or `no` when disabled.

## Unclean shutdown recovery

When Lima runs as a LaunchDaemon with `KeepAlive` enabled, launchd sends `SIGTERM` to the host
agent during system shutdown. Depending on how far shutdown progresses before the host agent
exits, the instance can enter one of two broken states on the next boot:

- **Orphaned VZ driver** — the host agent exits before it can stop the VM driver, leaving the
  driver process running with no host agent attached. Caused by launchd killing the host agent
  before it finishes its shutdown sequence.
- **Stale host agent socket** — the host agent exits cleanly but its `ha.pid` file is not
  removed before the system halts. On the next boot, the PID in `ha.pid` may be reused by an
  unrelated process, making the `ha.sock` unreachable. Caused by macOS halting the process
  before its deferred cleanup runs.

Lima detects and recovers from both states automatically on the next `limactl start`:

- For an orphaned driver, `limactl start` force-stops the driver process and starts cleanly.
- For a stale socket, `limactl start` removes the stale `ha.pid` and `ha.sock` files (without
  signaling the unrelated process) and starts cleanly.

No manual intervention is required when using `--keep-alive` (the default).

If you encounter either state manually (e.g. after `kill -9` on the host agent), you can recover with:

```bash
limactl stop --force <instance>
limactl start <instance>
```

### VZ graceful shutdown fallback

On some macOS versions the VZ `CanRequestStop()` API returns false because no
`RequestStopHandler` was registered on the VM configuration. When this happens, Lima falls
back to requesting shutdown via SSH so the guest exits cleanly rather than being killed
when the host agent exits. This is transparent and requires no user action.

## Lima < 2.2

Use `limactl start-at-login` (equivalent to `limactl autostart enable --condition=login`):

```bash
# Register
limactl start-at-login default

# Unregister
limactl start-at-login --enabled=false default
```
