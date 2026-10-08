---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/prune.go#pruneAction
  - kind: code
    role: implementation
    target: cmd/limactl/prune.go#knownLocations
  - kind: code
    role: implementation
    target: pkg/downloader/downloader.go#RemoveAllCacheDir
  - kind: code
    role: implementation
    target: pkg/downloader/downloader.go
---

# Prune cached downloads

Remove downloaded images and other cached downloads (`limactl prune`, "Prune garbage objects"), either all of them or only those no instance or template refers to.
