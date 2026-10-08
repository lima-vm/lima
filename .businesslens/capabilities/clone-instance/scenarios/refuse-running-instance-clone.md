---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to clone a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "cannot clone a running instance"
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse to clone a running instance

## Trigger

The instance to clone is running.

## Outcome

Nothing is created.
