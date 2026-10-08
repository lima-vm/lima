---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to rename a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "cannot rename a running instance"
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse to rename a running instance

## Trigger

The instance to rename is running.

## Outcome

The instance is unchanged.
