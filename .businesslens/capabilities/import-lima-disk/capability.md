---
availability:
  - place: limactl
domain: lima-disks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskImportAction
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

# Import a Lima disk

Make a copy of an existing qcow2 or raw disk image a Lima disk.
