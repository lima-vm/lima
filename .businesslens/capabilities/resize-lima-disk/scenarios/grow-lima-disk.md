---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to resize a Lima disk to a larger size
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name, Size]}
    contexts:
      cli:
        place: limactl
  - text: The Product grows the Lima disk
    kind: product
    actor: user
    entities:
      - {entity: lima-disk, effect: changes, facts: [Size]}
    contexts:
      cli:
        place: limactl
---

# Grow a disk

## Trigger

The User runs `limactl disk resize NAME --size SIZE`.

## Outcome

The Lima disk has the new size.

## Edge cases

- A Lima disk held by a running instance is refused until the instance stops.
