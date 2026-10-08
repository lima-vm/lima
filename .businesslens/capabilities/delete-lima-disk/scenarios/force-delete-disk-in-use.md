---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to delete a Lima disk by force
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes the Lima disk even though it is held
    kind: product
    actor: user
    entities:
      - {entity: lima-disk, effect: removes, from: In use}
    contexts:
      cli:
        place: limactl
---

# Delete a disk in use by force

## Trigger

The User runs `limactl disk delete --force NAME`.

## Outcome

The Lima disk no longer exists, even if an instance held it or refers to it.

## Edge cases

- Forcing does not stop the instance that holds the disk.
