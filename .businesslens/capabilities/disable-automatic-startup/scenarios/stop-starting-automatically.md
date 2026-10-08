---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to unregister an instance from automatic startup
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes the instance's registration
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, facts: [Automatic startup]}
    contexts:
      cli:
        place: limactl
---

# Stop starting an instance automatically

## Trigger

The User runs `limactl autostart disable NAME`.

## Outcome

The instance starts only when asked.

## Edge cases

- An instance that is not registered is reported and left unchanged.
- On macOS a registration at login is removed before one at boot; running the command again removes the other.
