---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks for a screenshot of a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product saves the display as PNG or BMP
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Capture the display

## Trigger

The User runs `limactl screenshot NAME -o FILE.png`.

## Outcome

The image holds the instance's display.
