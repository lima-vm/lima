---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/factory-reset.go#factoryResetAction
---

# Factory reset an instance

Return an instance to the state it had before its first start: it is stopped by force, and its disk, logs, events and connection files are removed while its configuration is kept, so the next start boots a fresh guest from the images.
