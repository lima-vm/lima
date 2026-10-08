---
kind: edge
routes:
  cli: Command line
steps:
  - text: A running instance's host agent stops answering, or one of its processes exits without the other
    kind: condition
    unattended: true
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl::instance-list
  - text: The Product finds the instance inconsistent when it inspects it and records why
    kind: product
    entities:
      - {entity: instance, effect: changes, from: Running, to: Broken, facts: [Errors]}
    contexts:
      cli:
        place: limactl::instance-list
  - text: The instance is listed as Broken with its errors
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Errors]}
    contexts:
      cli:
        place: limactl::instance-list
---

# Show an instance that has broken

## Trigger

An instance's processes or configuration are found inconsistent when instances are listed.

## Outcome

The User sees the instance as Broken with its errors, and can stop it by force or start it to recover.

## Edge cases

- A directory whose configuration cannot be read is listed as Broken too.
