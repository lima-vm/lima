---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to edit a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "cannot edit a running instance"
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse to edit a running instance

## Trigger

The User runs `limactl edit` on a running instance.

## Outcome

The instance is unchanged.
