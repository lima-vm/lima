---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to unlock a Lima disk
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name, In use by]}
    contexts:
      cli:
        place: limactl
  - text: The Product releases the Lima disk from its holder
    kind: product
    actor: user
    entities:
      - {entity: lima-disk, effect: changes, from: In use, to: Available, facts: [In use by]}
    contexts:
      cli:
        place: limactl
---

# Unlock a disk

## Trigger

The User runs `limactl disk unlock NAME`.

## Outcome

The Lima disk is available to other instances.

## Edge cases

- A Lima disk that is not held, or a name with no disk, is skipped with a warning.
