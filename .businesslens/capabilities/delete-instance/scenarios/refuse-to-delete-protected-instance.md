---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to delete a protected instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "instance is protected to prohibit accidental removal" and suggests `limactl unprotect`
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Protection]}
    contexts:
      cli:
        place: limactl
---

# Refuse to delete a protected instance

## Trigger

The User runs `limactl delete`, with or without `--force`, on a protected instance.

## Outcome

The instance is unchanged.
