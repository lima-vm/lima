---
domain: lima-disks
references:
  - kind: code
    role: implementation
    target: pkg/store/disk.go#InspectDisk
  - kind: code
    role: implementation
    target: pkg/store/disk.go#LockForInstance
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskListAction
  - kind: doc
    role: context
    target: website/content/en/docs/config/disk.md
  - kind: code
    role: implementation
    target: pkg/store/disk.go#Disk
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go
---

# Lima disk

An extra virtual disk kept apart from any instance, which instances attach by name and which survives the instances that use it.

## Information kept

- **Name** — the disk's unique name
- **Size** — its size
- **Format** — the image format it is stored in, `qcow2` or `raw`
- **Directory** — where its image is kept under the Lima home directory
- **In use by** — the instance currently holding it, if any

## States

### Available

No instance holds the disk.

### In use

One instance holds the disk, from when it starts until it stops or the disk is
unlocked.
