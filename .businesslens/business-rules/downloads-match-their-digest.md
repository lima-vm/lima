---
appliesTo:
  - type: entity
    id: cached-download
  - type: capability
    id: create-instance
references:
  - kind: code
    role: implementation
    target: pkg/downloader/downloader.go#validateCachedDigest
---

# A download is used only when it matches its declared digest

When a template declares a digest for an image, kernel, initrd or archive, Lima
uses the download only if its content matches that digest.
