# Experimental features

The following features are experimental and subject to change:

- `mountType: 9p`
- `mountType: virtiofs` on Linux
- `vmType: vz` and relevant configurations (`mountType: virtiofs`, `rosetta`, `[]networks.vzNAT`)
- `arch: riscv64`
- `video.display: vnc` and relevant configuration (`video.vnc.display`)
- `mode: user-v2` in `networks.yml` and relevant configuration in `lima.yaml`
- `audio.device`
- `arch: armv7l`

The following commands are experimental and subject to change:

- `limactl (create|start|edit) --set=<YQ EXPRESSION>`
- `limactl (create|start|edit) --network=<NETWORK>`
- `limactl snapshot *`
