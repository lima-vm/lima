---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks for a screenshot
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The instance is not running, has no display, or its driver cannot capture one, and the Product refuses
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [VM type]}
    contexts:
      cli:
        place: limactl
---

# Refuse a screenshot the instance cannot give

## Trigger

The instance cannot provide a screenshot, or the output name does not end in `.png` or `.bmp`.

## Outcome

Nothing is written.
