---
entities:
  - entity: instance
    shows: [Name]
  - entity: instance-event
    shows: [Time, Status, Errors, SSH local port, Provisioning progress, Port forward]
entryPoints:
  - limactl: limactl watch
references:
  - kind: code
    role: implementation
    target: cmd/limactl/watch.go#watchAction
---

# Instance events

The events of the named instances, or of every instance including those
created while watching, as they happen and, when asked, from before.
