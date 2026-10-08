---
kind: edge
routes:
  cli: Command line
steps:
  - text: The guest agent of a running instance is not running
    kind: condition
    unattended: true
    entities:
      - {entity: instance, effect: reads, facts: [Plain mode]}
    contexts:
      cli:
        place: limactl
  - text: The Product forwards only the rules marked static
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Port forwards]}
    contexts:
      cli:
        place: limactl
---

# No dynamic forwarding without the guest agent

## Trigger

The instance runs in plain mode, or its guest agent never started.

## Outcome

Only static forwards work.
