---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to delete a running instance without force
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses, expecting the instance to be stopped
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse to delete a running instance without force

## Trigger

The User runs `limactl delete` without `--force` on an instance that is not stopped.

## Outcome

The instance is unchanged, and the instances named after it are not deleted.
