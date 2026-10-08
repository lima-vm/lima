---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to protect one or more instances
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product marks each instance protected
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, facts: [Protection]}
    contexts:
      cli:
        place: limactl
---

# Protect an instance

## Trigger

The User runs `limactl protect NAME…`.

## Outcome

The instance cannot be deleted or factory reset until it is unprotected.

## Edge cases

- An instance already protected is skipped with a warning.
- A name with no instance ends the command; the instances before it stay protected.
