---
title: Intel-on-ARM and ARM-on-Intel
weight: 20
---

Lima supports two modes for running Intel-on-ARM and ARM-on-Intel:
- [Slow mode](#slow-mode)
- [Fast mode](#fast-mode)
- [Fast mode 2](#fast-mode-2)

## [Slow mode: Intel VM on ARM Host / ARM VM on Intel Host](#slow-mode)

Lima can run a VM with a foreign architecture, just by specifying `arch` in the YAML.

```yaml
arch: "x86_64"
# arch: "aarch64"

images:
  - location: "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img"
    arch: "x86_64"
  - location: "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-arm64.img"
    arch: "aarch64"

# Disable mounts and containerd, otherwise booting up may time out if the host is slow
mounts: []
containerd:
  system: false
  user: false
```

Running a VM with a foreign architecture is extremely slow.
Consider using [Fast mode](#fast-mode) or [Fast mode 2](#fast-mode-2) whenever possible.

## [Fast mode: Intel containers on ARM VM on ARM Host / ARM containers on Intel VM on Intel Host](#fast-mode)

This mode uses QEMU User Mode Emulation.
QEMU User Mode Emulation is significantly faster than QEMU System Mode Emulation, but it often sacrifices compatibility.

Set up:
```bash
lima sudo systemctl start containerd
lima sudo nerdctl run --privileged --rm tonistiigi/binfmt:qemu-v7.0.0-28@sha256:66e11bea77a5ea9d6f0fe79b57cd2b189b5d15b93a2bdb925be22949232e4e55 --install all
```

Run containers:
```console
$ lima nerdctl run --platform=amd64 --rm alpine uname -m
x86_64

$ lima nerdctl run --platform=arm64 --rm alpine uname -m
aarch64
```

Build and push container images:
```console
$ lima nerdctl build --platform=amd64,arm64 -t example.com/foo:latest .
$ lima nerdctl push --all-platforms example.com/foo:latest
```

See also https://github.com/containerd/nerdctl/blob/master/docs/multi-platform.md

## [Fast mode 2 (Rosetta): Intel containers on ARM VM on ARM Host](#fast-mode-2)

> **Warning**
> "vz" mode, including support for Rosetta, is experimental (will graduate from experimental in Lima v1.0)

| ⚡ Requirement | Lima >= 0.14, macOS >= 13.0, ARM |
|-------------------|----------------------------------|

[Rosetta](https://developer.apple.com/documentation/virtualization/running_intel_binaries_in_linux_vms_with_rosetta) is known to be much faster than QEMU User Mode Emulation.
Rosetta is available for [VZ](../vmtype/#vz) instances on ARM hosts.

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --vm-type=vz --rosetta
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
vmType: "vz"
rosetta:
  # Enable Rosetta for Linux.
  # Hint: try `softwareupdate --install-rosetta` if Lima gets stuck at `Installing rosetta...`
  enabled: true
  # Register rosetta to /proc/sys/fs/binfmt_misc
  binfmt: true
```
{{% /tab %}}
{{< /tabpane >}}
