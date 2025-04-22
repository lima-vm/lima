---
title: Experimental features
weight: 10
---


The following features are experimental and subject to change:

- `mountType: virtiofs` on Linux
- `vmType: wsl2` and relevant configurations (`mountType: wsl2`)
- `arch: riscv64`, `arch: armv7l`, and `arch: s390x`
- `video.display: vnc` and relevant configuration (`video.vnc.display`)
- `audio.device`
- `mountInotify: true`

The following commands are experimental and subject to change:

- `limactl snapshot *`
- `limactl tunnel`
- `limactl template *`

## Graduated

The following features were experimental in the past:

### Until v1.0

- `mountType: 9p`
- `vmType: vz` and relevant configurations (`mountType: virtiofs`, `rosetta`, `[]networks.vzNAT`)
- `mode: user-v2` in `networks.yml` and relevant configuration in `lima.yaml`
