---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/clone.go#cloneOrRenameAction
  - kind: code
    role: implementation
    target: pkg/instance/clone.go#CloneOrRename
  - kind: code
    role: implementation
    target: cmd/limactl/clone.go#newRenameCommand
  - kind: code
    role: implementation
    target: pkg/limatype/filenames/filenames.go#SkipOnClone
---

# Rename an instance

Give an instance that is not running a new name, optionally changing its configuration with edit flags. The renamed instance is no longer protected.
