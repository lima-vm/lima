---
availability:
  - place: limactl
domain: lima-disks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskDeleteAction
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

# Delete a Lima disk

Remove Lima disks that no instance holds or refers to, or any Lima disk by force.
