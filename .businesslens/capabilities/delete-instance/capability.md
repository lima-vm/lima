---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/delete.go#deleteAction
  - kind: code
    role: implementation
    target: pkg/instance/delete.go#Delete
---

# Delete an instance

Remove instances and everything Lima keeps for them, including their disks, snapshots, events and automatic-startup registration. The Lima disks they attached are kept.
