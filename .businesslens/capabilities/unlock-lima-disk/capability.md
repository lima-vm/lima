---
availability:
  - place: limactl
domain: lima-disks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskUnlockAction
  - kind: code
    role: implementation
    target: pkg/store/disk.go#Unlock
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

# Unlock a Lima disk

Release a Lima disk still held by an instance that is no longer running, for example after a crash.
