---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks for a tunnel to a stopped instance
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

# Refuse a tunnel to a stopped instance

## Trigger

The instance is stopped, or no instance has the name.

## Outcome

No tunnel is created.
