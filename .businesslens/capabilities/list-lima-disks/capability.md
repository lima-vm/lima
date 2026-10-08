---
availability:
  - place: limactl
domain: lima-disks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskListAction
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

# List Lima disks

Show every Lima disk, or those named, with the instance holding it.
