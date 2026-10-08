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
    target: pkg/limatype/filenames/filenames.go
---

# Clone an instance

Copy an instance that is not running, with its configuration and disk, into a new instance under another name, optionally changing its configuration with edit flags. The copy is never protected and keeps referring to the same Lima disks by name.
