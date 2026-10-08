---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to unlock a Lima disk
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: Its holder is running or has errors, and the Product skips the Lima disk with a warning
    kind: condition
    entities:
      - {entity: lima-disk, effect: reads, facts: [In use by]}
    contexts:
      cli:
        place: limactl
---

# Refuse to unlock a disk a running instance uses

## Trigger

The Lima disk is held by a running instance, or by one whose inspection reports errors.

## Outcome

The Lima disk stays in use.
