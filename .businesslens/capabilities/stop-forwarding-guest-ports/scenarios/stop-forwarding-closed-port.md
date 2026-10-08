---
kind: primary
routes:
  cli: Command line
steps:
  - text: A program in a running instance stops listening on a forwarded port
    kind: condition
    unattended: true
    entities:
      - {entity: instance, effect: reads, facts: [Forwarded ports]}
    contexts:
      cli:
        place: limactl
  - text: The Product stops forwarding the host port to it
    kind: product
    entities:
      - {entity: instance, effect: changes, facts: [Forwarded ports]}
    contexts:
      cli:
        place: limactl
---

# Stop forwarding a port the guest closes

## Trigger

The guest agent reports that a forwarded port is no longer listened on.

## Outcome

The host port is no longer forwarded and is free for other programs.
