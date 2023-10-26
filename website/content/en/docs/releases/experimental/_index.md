---
title: Experimental features
weight: 10
---


The following features are experimental and subject to change:

- `mountType: 9p` (will graduate from experimental in Lima v1.0)
- `mountType: virtiofs` on Linux
- `vmType: vz` and relevant configurations (`mountType: virtiofs`, `rosetta`, `[]networks.vzNAT`)
  (will graduate from experimental in Lima v1.0)
- `vmType: wsl2` and relevant configurations (`mountType: wsl2`)
- `arch: riscv64`
- `video.display: vnc` and relevant configuration (`video.vnc.display`)
- `mode: user-v2` in `networks.yml` and relevant configuration in `lima.yaml`
- `audio.device`
- `arch: armv7l`
- `mountInotify: true`

The following commands are experimental and subject to change:

- `limactl snapshot *`
