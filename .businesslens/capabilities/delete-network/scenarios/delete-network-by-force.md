---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to delete a network with `--force`
    kind: actor
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes the network
    kind: product
    actor: user
    entities:
      - {entity: network, effect: removes}
    contexts:
      cli:
        place: limactl
---

# Delete a network

## Trigger

The User runs `limactl network delete --force NAME`.

## Outcome

The network no longer exists.
