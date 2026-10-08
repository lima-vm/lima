---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User runs `limactl NAME` for an installed plugin
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product runs the plugin executable with the remaining arguments and exits with its status
    kind: product
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
---

# Run a plugin command

## Trigger

The User runs a plugin command such as `limactl mcp`.

## Outcome

The plugin has run, and its exit status is the command's.

## Edge cases

- When two plugins share a name, the first one found is used.
