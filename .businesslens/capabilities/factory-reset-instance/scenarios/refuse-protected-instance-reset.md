---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to factory reset a protected instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "instance is protected to prohibit accidental factory-reset"
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Protection]}
    contexts:
      cli:
        place: limactl
---

# Refuse to factory reset a protected instance

## Trigger

The instance is protected.

## Outcome

The instance is unchanged.
