---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to watch the events of instances
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl::instance-events
  - text: The User reads each instance event as it occurs until they interrupt the command
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
      - {entity: instance-event, effect: reads, facts: [Time, Status, Errors, SSH local port, Provisioning progress, Port forward]}
    contexts:
      cli:
        place: limactl::instance-events
---

# Watch events from instances

## Trigger

The User runs `limactl watch`.

## Outcome

The User has followed the instances' events.

## Edge cases

- `--history` also prints the events from before watching began; `--json` prints one JSON object per event.
- An instance that cannot be inspected is reported while the others are still watched.
