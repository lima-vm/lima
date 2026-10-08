---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to delete a network without `--force`
    kind: actor
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: 'The Product refuses: deleting a network currently always requires `--force`'
    kind: condition
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse to delete a network without force

## Trigger

`--force` was not given.

## Outcome

The network is unchanged.
