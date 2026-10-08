---
availability:
  - place: limactl
domain: lima-disks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskResizeAction
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go
  - kind: code
    role: implementation
    target: pkg/store/disk.go
  - kind: code
    role: implementation
    target: pkg/imgutil/proxyimgutil/proxyimgutil.go
---

# Resize a Lima disk

Grow a Lima disk that no running instance uses.
