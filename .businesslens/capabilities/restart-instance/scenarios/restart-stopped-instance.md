---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to restart a stopped instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product warns that the instance is not running and starts it
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
---

# Restart an instance that is not running

## Trigger

The User runs `limactl restart` on a stopped instance.

## Outcome

The instance is running.
