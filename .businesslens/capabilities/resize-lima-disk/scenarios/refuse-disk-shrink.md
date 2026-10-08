---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to resize a Lima disk to a smaller size
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: 'The Product refuses: disk shrinking is unavailable'
    kind: condition
    entities:
      - {entity: lima-disk, effect: reads, facts: [Size]}
    contexts:
      cli:
        place: limactl
---

# Refuse to shrink a disk

## Trigger

The size asked for is less than the current size.

## Outcome

The Lima disk is unchanged.
