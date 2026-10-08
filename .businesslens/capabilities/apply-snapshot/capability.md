---
availability:
  - place: limactl
domain: snapshots
references:
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go#snapshotApplyAction
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go#findSnapshotIDByTag
  - kind: code
    role: implementation
    target: pkg/snapshot/snapshot.go#Load
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go
  - kind: code
    role: implementation
    target: pkg/snapshot/snapshot.go
  - kind: code
    role: implementation
    target: pkg/driver/qemu/qemu.go
  - kind: code
    role: implementation
    target: pkg/driver/vz/vz_driver_darwin.go
---

# Apply a snapshot

Return an instance to the state saved in one of its snapshots. Taking, applying, deleting and listing snapshots is experimental and available for QEMU and krunkit instances; krunkit instances must be stopped and their snapshots cover only the boot disk.
