---
entities:
  - entity: snapshot
    shows: [ID, Tag, Created]
entryPoints:
  - limactl: limactl snapshot list
references:
  - kind: code
    role: implementation
    target: cmd/limactl/snapshot.go#printSnapshots
---

# Snapshot list

The snapshots of one instance.
