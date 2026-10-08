---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to delete a snapshot of an instance by its tag or ID
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
      - {entity: snapshot, effect: reads, facts: [Tag, ID]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes the snapshot
    kind: product
    actor: user
    entities:
      - {entity: snapshot, effect: removes}
    contexts:
      cli:
        place: limactl
---

# Delete a snapshot

## Trigger

The User runs `limactl snapshot delete NAME --tag TAG`, or with `--id`.

## Outcome

The instance no longer has the snapshot.

## Edge cases

- A tag the instance does not have, or that several snapshots share, is refused.
