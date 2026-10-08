---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to create an instance under a name an instance already has
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "instance already exists" and creates nothing
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse a name already in use

## Trigger

The User runs `limactl create --name=NAME` while an instance NAME exists.

## Outcome

Nothing is created; the existing instance is unchanged.

## Edge cases

- A name that is not a valid identifier, ends in `.yaml` or `.yml`, or is too long for the instance's sockets is refused the same way.
