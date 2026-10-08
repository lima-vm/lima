---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks for the sudoers content
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product prints the content for the networks configured in `networks.yaml`
    kind: product
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name, Mode]}
    contexts:
      cli:
        place: limactl
---

# Print the sudoers content

## Trigger

The User runs `limactl sudoers` and installs the output with sudo.

## Outcome

The User has the content to install.

## Edge cases

- `--block-device` adds entries for those host disk devices and the current user; other users' entries are not regenerated.
- With block devices, networks that fail validation are left out with a warning that replacing the installed content removes their grants.
- Without block devices, a network configuration that fails validation ends the command with an error.
