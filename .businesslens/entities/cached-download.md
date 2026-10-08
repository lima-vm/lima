---
references:
  - kind: code
    role: implementation
    target: pkg/downloader/downloader.go#Download
  - kind: code
    role: implementation
    target: pkg/downloader/downloader.go#CacheEntries
  - kind: doc
    role: context
    target: website/content/en/docs/dev/internals.md
  - kind: code
    role: implementation
    target: pkg/downloader/downloader.go
  - kind: code
    role: implementation
    target: cmd/limactl/prune.go#knownLocations
---

# Cached download

An image, kernel, initrd, firmware or nerdctl archive Lima downloaded once and keeps in the User's cache directory, so that later instances reuse it.

## Information kept

- **URL** — where it was downloaded from
- **Data** — the downloaded content, with a converted raw copy when a driver needs one
- **Digest** — the digest of the content
