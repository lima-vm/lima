---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to apply a snapshot by a tag the instance does not have
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
      - {entity: snapshot, effect: reads, facts: [Tag]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "snapshot tag not found", or "not unique" when several snapshots share it
    kind: condition
    entities:
      - {entity: snapshot, effect: reads, facts: [Tag]}
    contexts:
      cli:
        place: limactl
---

# Refuse a snapshot that cannot be found

## Trigger

No snapshot of the instance has the tag, or several do.

## Outcome

The instance is unchanged.

## Edge cases

- Neither a tag nor an ID is refused with "expected ID or tag".
