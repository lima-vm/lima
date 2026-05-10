---
title: Experimental features
weight: 10
---


The following features are experimental and subject to change:

- `mountType: virtiofs` on Linux
- `vmType: wsl2` and relevant configurations (`mountType: wsl2`)
- `arch`: `riscv64`, `armv7l`, `s390x`, and `ppc64le`
- `video.display: vnc` and relevant configuration (`video.vnc.display`)
- `audio.device`
- `mountInotify: true`
- `External drivers`: building and using drivers as separate executables (see [Virtual Machine Drivers](../dev/drivers))
- [`vmType: krunkit`](../config/vmtype/krunkit.md)
- [`github` URL scheme](../templates/github.md): referencing templates on GitHub with `github:` URLs
- [macOS guests](../usage/guests/macos.md)
- [FreeBSD guests](../usage/guests/freebsd.md)

The following commands are experimental and subject to change:

- `limactl snapshot *`
- `limactl tunnel`
- `limactl template *`
- `limactl mcp *`

## Graduated

The following features were experimental in the past:

### Until v1.0

- `mountType: 9p`
- `vmType: vz` and relevant configurations (`mountType: virtiofs`, `rosetta`, `[]networks.vzNAT`)
- `mode: user-v2` in `networks.yml` and relevant configuration in `lima.yaml`
