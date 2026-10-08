---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks for the saved states of an instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The User reads each snapshot's ID, tag and creation time
    kind: actor
    actor: user
    entities:
      - {entity: snapshot, effect: reads, facts: [ID, Tag, Created]}
    contexts:
      cli:
        place: limactl::snapshot-list
---

# List an instance's snapshots

## Trigger

The User runs `limactl snapshot list NAME`.

## Outcome

The User sees the instance's snapshots.

## Edge cases

- `--quiet` prints only the tags, and `--json` prints JSON.
- An instance whose VM type does not support snapshots is refused.
