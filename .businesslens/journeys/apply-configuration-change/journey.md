---
actors:
  - user
references:
  - kind: code
    role: implementation
    target: cmd/limactl/edit.go#editAction
  - kind: code
    role: implementation
    target: cmd/limactl/edit.go#askWhetherToStart
---

# Apply a configuration change

## Goal

Change a stopped instance's configuration and have it running with the change.

## Success criterion

The instance runs with the changed configuration.
