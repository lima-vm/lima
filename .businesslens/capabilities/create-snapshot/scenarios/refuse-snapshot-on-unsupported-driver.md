---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to save the state of a VZ, WSL2 or HCS instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "unimplemented"
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [VM type]}
    contexts:
      cli:
        place: limactl
---

# Refuse a snapshot the driver cannot take

## Trigger

The instance's VM type does not support snapshots.

## Outcome

No snapshot is created.
