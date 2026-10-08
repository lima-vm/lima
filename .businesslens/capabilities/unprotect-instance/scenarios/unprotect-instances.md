---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to unprotect one or more instances
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes each instance's protection
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, facts: [Protection]}
    contexts:
      cli:
        place: limactl
---

# Unprotect an instance

## Trigger

The User runs `limactl unprotect NAME…`.

## Outcome

The instance can be deleted and factory reset again.

## Edge cases

- An instance that is not protected is skipped with a warning.
