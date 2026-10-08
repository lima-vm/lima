---
availability:
  - place: limactl
domain: snapshots
references:
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go#snapshotCreateAction
  - kind: code
    role: implementation
    target: pkg/snapshot/snapshot.go#Save
  - kind: code
    role: implementation
    target: pkg/driver/qemu/qemu.go#Save
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

# Create a snapshot

Save an instance's current state under a tag. Taking, applying, deleting and listing snapshots is experimental and available for QEMU and krunkit instances; krunkit instances must be stopped and their snapshots cover only the boot disk.
