# Experimental features

The following features are experimental and subject to change:

- `mountType: 9p`
- `vmType: vz` and relevant configurations (`mountType: virtiofs`, `rosetta`, `[]networks.vzNAT`)
- `arch: riscv64`
- `video.display: vnc` and relevant configuration (`video.vnc.display`)
- `mode: user-v2` in `networks.yml` and relevant configuration in `lima.yaml` 
- `audio.device`

The following flags are experimental and subject to change:

- `start --set`, yq expression
- `edit --set`, yq expression
