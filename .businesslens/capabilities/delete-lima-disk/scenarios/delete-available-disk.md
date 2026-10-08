---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to delete one or more Lima disks
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes each Lima disk that is not in use and that no instance refers to
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Additional disks]}
      - {entity: lima-disk, effect: removes, from: Available}
    contexts:
      cli:
        place: limactl
---

# Delete a disk

## Trigger

The User runs `limactl disk delete NAME`.

## Outcome

The Lima disk no longer exists.

## Edge cases

- A Lima disk that stopped instances still refer to is skipped with a warning.
- A name with no Lima disk is ignored with a warning.
