---
domain: snapshots
references:
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go#printSnapshots
  - kind: code
    role: implementation
    target: pkg/snapshot/snapshot.go#Save
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go
  - kind: code
    role: implementation
    target: pkg/driver/qemu/qemu.go#List
---

# Snapshot

A saved state of an instance that the User can return the instance to later.

## Information kept

- **ID** — the identifier the virtualization driver gives the snapshot
- **Tag** — the name the User gave the snapshot
- **Created** — when the snapshot was taken, where the driver reports it
