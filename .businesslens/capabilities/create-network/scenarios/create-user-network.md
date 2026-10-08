---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to create a network with a name, mode and gateway
    kind: actor
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product adds the network
    kind: product
    actor: user
    entities:
      - {entity: network, effect: creates, facts: [Name, Mode, Gateway]}
    contexts:
      cli:
        place: limactl
---

# Create a network with a gateway

## Trigger

The User runs `limactl network create NAME --gateway 192.168.42.1/24`.

## Outcome

Instances can join the network by name.

## Edge cases

- A gateway without a netmask is taken as /24.
- The Product does not check whether the address range collides with another network.
