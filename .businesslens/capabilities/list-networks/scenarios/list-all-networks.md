---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to list networks
    kind: actor
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl::network-list
  - text: The User reads each network's name, mode, gateway and interface
    kind: actor
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name, Mode, Gateway, Interface]}
    contexts:
      cli:
        place: limactl::network-list
---

# List networks

## Trigger

The User runs `limactl network list`.

## Outcome

The User sees the networks.

## Edge cases

- `--json` prints JSON.
- Names that match no network are reported with a warning.
- When `networks.yaml` does not exist yet, Lima writes it with the networks it ships.
