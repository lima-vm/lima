---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to start an instance that attaches a Lima disk
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Additional disks]}
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Lima disk is already in use elsewhere, and the Product refuses to start
    kind: condition
    entities:
      - {entity: lima-disk, effect: reads, facts: [In use by]}
    contexts:
      cli:
        place: limactl
---

# Refuse a disk another instance holds

## Trigger

A Lima disk the instance attaches is held by another instance.

## Outcome

The instance does not start.
