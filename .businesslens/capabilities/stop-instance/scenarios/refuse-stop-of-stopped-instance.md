---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to stop an instance that is not running
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses, suggesting `limactl stop -f`
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse to stop an instance that is not running

## Trigger

The User runs `limactl stop` without `--force` on a stopped or broken instance.

## Outcome

The instance is unchanged.
