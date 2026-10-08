---
actors:
  - user
references:
  - kind: code
    role: implementation
    target: cmd/limactl/clone.go#cloneOrRenameAction
---

# Run a renamed instance

## Goal

Give a stopped instance a new name and have it running under that name.

## Success criterion

The instance is running under its new name.
