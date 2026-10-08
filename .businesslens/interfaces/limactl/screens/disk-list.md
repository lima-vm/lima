---
entities:
  - entity: lima-disk
    shows: [Name, Size, Format, Directory, In use by]
entryPoints:
  - limactl: limactl disk list
references:
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskListAction
---

# Disk list

Every Lima disk, or those named, with the instance holding it.
