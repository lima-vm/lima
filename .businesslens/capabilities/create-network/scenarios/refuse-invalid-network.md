---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to create a network
    kind: actor
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The name is in use, a bridged network lacks an interface or has a gateway, or another mode lacks a valid gateway or has an interface, and the Product refuses
    kind: condition
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse a network that is incomplete or exists

## Trigger

The network cannot be created as asked.

## Outcome

No network is added.

## Edge cases

- A loopback or unspecified gateway address is refused.
