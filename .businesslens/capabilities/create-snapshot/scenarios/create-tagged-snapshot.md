---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to save an instance's state under a tag
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product saves the instance's state as a snapshot with that tag
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Guest disk]}
      - {entity: snapshot, effect: creates, facts: [ID, Tag, Created]}
    contexts:
      cli:
        place: limactl
---

# Create a tagged snapshot

## Trigger

The User runs `limactl snapshot create NAME --tag TAG`.

## Outcome

The instance has a snapshot with that tag.

## Edge cases

- A snapshot without a tag is refused with "expected tag".
- A krunkit instance that is running is refused until it stops.
