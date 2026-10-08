---
availability:
  - place: limactl
domain: snapshots
references:
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go
  - kind: code
    role: implementation
    target: pkg/snapshot/snapshot.go
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go#snapshotDeleteAction
  - kind: code
    role: implementation
    target: pkg/snapshot/snapshot.go#Del
  - kind: code
    role: implementation
    target: pkg/driver/qemu/qemu.go
  - kind: code
    role: implementation
    target: pkg/driver/vz/vz_driver_darwin.go
---

# Delete a snapshot

Remove one of an instance's snapshots. Taking, applying, deleting and listing snapshots is experimental and available for QEMU and krunkit instances; krunkit instances must be stopped and their snapshots cover only the boot disk.
