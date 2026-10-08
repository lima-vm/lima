---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to apply a snapshot of an instance by its tag or ID
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
      - {entity: snapshot, effect: reads, facts: [Tag, ID]}
    contexts:
      cli:
        place: limactl
  - text: The Product restores the instance to the snapshot's state
    kind: product
    actor: user
    entities:
      - {entity: snapshot, effect: reads, facts: [ID]}
      - {entity: instance, effect: changes, facts: [Guest disk]}
    contexts:
      cli:
        place: limactl
---

# Apply a snapshot

## Trigger

The User runs `limactl snapshot apply NAME --tag TAG`, or with `--id`.

## Outcome

The instance's disk holds the state saved in the snapshot.
