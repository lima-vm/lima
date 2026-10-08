---
appliesTo:
  - type: entity
    id: lima-disk
  - type: capability
    id: start-instance
references:
  - kind: code
    role: implementation
    target: pkg/store/disk.go#LockForInstance
---

# A Lima disk is used by one instance at a time

A Lima disk is held by the instance that attaches it from the moment that
instance starts until it stops or the disk is unlocked, and no other instance
can start with it meanwhile. The same instance starting again takes its own
hold back.
