---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to create a bridged network on a host interface
    kind: actor
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product adds the network on that interface
    kind: product
    actor: user
    entities:
      - {entity: network, effect: creates, facts: [Name, Mode, Interface]}
    contexts:
      cli:
        place: limactl
---

# Create a bridged network

## Trigger

The User runs `limactl network create NAME --mode bridged --interface en0`.

## Outcome

Instances can join the network by name.
