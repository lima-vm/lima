---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to delete a Lima disk that is in use
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "cannot delete disk in use", naming its holder
    kind: condition
    entities:
      - {entity: lima-disk, effect: reads, facts: [In use by]}
    contexts:
      cli:
        place: limactl
---

# Refuse to delete a disk in use

## Trigger

The Lima disk is held by an instance and `--force` was not given.

## Outcome

The Lima disk is unchanged.
