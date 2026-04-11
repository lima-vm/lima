---
title: VZ
weight: 2
---

| ⚡ Requirement | Lima >= 0.14, macOS >= 13.0 |
|-------------------|-----------------------------|

"vz" option makes use of native virtualization support provided by macOS Virtualization.Framework.

"vz" has been the default driver for macOS hosts since Lima v1.0.

An example configuration (no need to be specified manually):
{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=vz
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "vz"

base:
- template:_images/ubuntu
- template:_default/mounts
```
{{% /tab %}}
{{< /tabpane >}}

### Memory Ballooning

| ⚡ Requirement | Lima >= 2.0.0, macOS >= 13.0, VZ backend only |
|-------------------|--------------------------------------------|

Memory ballooning dynamically adjusts the guest VM's memory allocation based on actual
usage. When the guest is idle, unused memory is returned to the host. When the guest
needs more memory (detected via PSI — Pressure Stall Information), the balloon grows
automatically.

This is configured under `vmOpts.vz.memoryBalloon`:

```yaml
vmType: "vz"
memory: "8GiB"

vmOpts:
  vz:
    memoryBalloon:
      enabled: true
      min: "2GiB"              # Floor — balloon never shrinks below this
      idleTarget: "3GiB"       # Target when VM is idle
      growStepPercent: 25      # Grow by 25% of max per step
      shrinkStepPercent: 10    # Shrink by 10% of max per step
      highPressureThreshold: 0.88  # PSI some10 threshold to trigger grow
      lowPressureThreshold: 0.35   # PSI some10 threshold to allow shrink
      cooldown: "30s"          # Minimum time between balloon actions
      idleGracePeriod: "5m"    # Wait after boot before ballooning begins
```

When `enabled` is not specified, memory ballooning defaults to disabled. When enabled
with no other fields specified, sensible defaults are derived from the configured
`memory` value (e.g., `min` defaults to 25% of `memory`, `idleTarget` to 33%).

The balloon controller also monitors container CPU/IO activity and swap-in rates to
avoid shrinking memory during active workloads.

### Auto-Pause

| ⚡ Requirement | Lima >= 2.0.0, macOS >= 13.0, VZ backend, memoryBalloon enabled |
|-------------------|---------------------------------------------------------------|

Auto-pause suspends a VZ virtual machine after a period of inactivity and resumes
it transparently when user activity is detected (shell access, Docker commands, etc.).
Combined with memory ballooning, this can reduce a paused VM's physical memory footprint
by 90% or more.

This is configured under `vmOpts.vz.autoPause`:

```yaml
vmOpts:
  vz:
    memoryBalloon:
      enabled: true
    autoPause:
      enabled: true
      idleTimeout: "15m"     # Pause after 15 minutes idle (minimum: 1m)
      resumeTimeout: "30s"   # Maximum time to wait for resume (minimum: 5s)
```

Auto-pause requires `memoryBalloon.enabled: true` because the balloon controller
shrinks guest memory before pausing, which reduces the frozen memory footprint.

When a paused VM is accessed (e.g., via `limactl shell` or a Docker command), it
resumes automatically. `limactl ls` shows "Paused" status and physical memory usage
(e.g., `802MB/8GiB`) for paused instances.

#### Manual Pause and Resume

You can manually pause and resume instances with `limactl pause` and `limactl resume`:

```bash
limactl pause docker-vz       # Immediately pause the instance
limactl resume docker-vz      # Resume a paused instance
```

Manual pause triggers the same mechanism as the idle timeout — the VM is suspended
immediately. After a manual pause, the VM can be resumed by:
- `limactl resume`
- Any client connecting to a forwarded socket (e.g., `docker ps`)
- The auto-pause idle timer resets on resume, so the VM won't re-pause immediately

#### Automatic Resume on Socket Connection

When auto-pause is enabled and socket forwarding is configured (e.g., for Docker),
Lima automatically resumes the VM when any client connects to a forwarded socket.
This means standard tools like `docker ps`, `docker compose`, and `kubectl` work
seamlessly — no wrapper scripts or manual resume commands needed.

The VM will be resumed within ~100ms of the first connection attempt. Subsequent
connections while the VM is running proceed with zero overhead.

This works for any socket forwarded through Lima's `portForwards` configuration,
not just Docker. For example, containerd, Podman, or custom application sockets
all benefit from auto-resume.

After macOS sleep/wake, SSH tunnels are automatically refreshed so the first
command after opening the lid works without delay.

#### Multi-Signal Idle Detection

Auto-pause uses multiple signals to determine whether the VM is genuinely idle,
preventing incorrect pauses during active work:

| Signal | What it detects | Examples |
|--------|----------------|----------|
| **Active proxy connections** | Open client connections through forwarded sockets | `docker exec -it`, `docker logs -f`, dev containers, long-running CLI sessions |
| **Container CPU activity** | Running containers with measurable CPU usage (> 0.5%) | Builds (`make -j8`), database queries, webpack dev servers |
| **Container IO activity** | Running containers with changing IO bytes | Disk-intensive operations, log writes |
| **Discrete commands** | Individual client commands that connect and disconnect | `docker ps`, `docker build`, `kubectl get pods` |

**The VM stays running** as long as any of these signals are active. Only when all
signals are quiet for the full `idleTimeout` (default 15 minutes) does the VM pause.

**The VM pauses** when there are no open connections, no containers doing measurable
work, and no recent commands — for example, when you walk away from your desk and
all containers are idle.

#### Configuring Idle Signals

Individual idle detection signals can be toggled via `idleSignals`. All default to
`true` — omitting `idleSignals` gives the same behavior as the defaults.

```yaml
vmOpts:
  vz:
    autoPause:
      enabled: true
      idleTimeout: "15m"
      resumeTimeout: "30s"
      idleSignals:
        activeConnections: true       # Track open proxy connections (default: true)
        containerCPU: true            # Track container CPU activity (default: true)
        containerCPUThreshold: 0.5    # Min CPU% to consider active (default: 0.5, range: 0.0–100.0)
        containerIO: true             # Track container IO changes (default: true)
```

Common configurations:

| Use Case | Configuration |
|----------|--------------|
| **Disable IO tracking** (chatty-log containers) | `containerIO: false` |
| **Raise CPU threshold** (high-baseline workloads) | `containerCPUThreshold: 5.0` |
| **Containerd/Podman** (no container metrics) | `containerCPU: false, containerIO: false` |
| **Touch-only mode** (disable all BusyChecks) | `activeConnections: false, containerCPU: false, containerIO: false` |

Set `containerCPUThreshold` above the baseline CPU usage of any always-running
background containers (monitoring agents, log collectors) to prevent them from
blocking pause indefinitely.

**Known limitations:**
- Container metrics (CPU, IO) are **Docker-only**. Containerd/nerdctl and Podman
  users still benefit from active connection tracking, but background tasks
  without a persistent client connection are not detected.
- Container metrics require `memoryBalloon.enabled: true` (they piggyback on the
  balloon controller's metrics pipeline).
- SSH sessions via `limactl shell` are not tracked by the socket proxy. Users
  working directly in a shell can use `limactl resume` if needed.

### Memory-Optimized Template

For a ready-to-use configuration with Docker, memory ballooning, and auto-pause
pre-configured on Alpine Linux:

```bash
limactl create --name=docker-vz templates/experimental/alpine-docker-vz.yaml
limactl start docker-vz
```

See `templates/experimental/alpine-docker-vz.yaml` for the full configuration.

### Caveats
- "vz" option is only supported on macOS 13 or above
- Virtualization.framework doesn't support running "intel guest on arm" and vice versa

### Known Issues
- "vz" doesn't support `legacyBIOS: true` option, so guest machine like `centos-stream` and `oraclelinux-8` will not work on Intel Mac.
- When running lima using "vz", `${LIMA_HOME}/<INSTANCE>/serial.log` will not contain kernel boot logs
- On Intel Mac with macOS prior to 13.5, Linux kernel v6.2 (used by Ubuntu 23.04, Fedora 38, etc.) is known to be unbootable on vz.
  kernel v6.3 and later should boot, as long as it is booted via GRUB.
  https://github.com/lima-vm/lima/issues/1577#issuecomment-1565625668
  The issue is fixed in macOS 13.5.
