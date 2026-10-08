---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks for a shell under a name no instance has
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses, suggesting `limactl create`
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse a shell where there is no instance

## Trigger

No instance has the name given.

## Outcome

No shell is opened.
