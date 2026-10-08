---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks for a shell in a stopped instance without a terminal
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses, suggesting `limactl start`
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse a shell in a stopped instance

## Trigger

The instance is stopped and the User cannot be asked whether to start it.

## Outcome

No shell is opened.
