---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to register an instance for automatic startup
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The host is not macOS or Linux, or startup at boot is asked off macOS, and the Product refuses
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse automatic startup the host does not support

## Trigger

The host cannot start the instance as asked.

## Outcome

The instance is unchanged.

## Edge cases

- A name with no instance is refused with "instance not found".
