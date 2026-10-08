---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks how to reach an instance over SSH
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product warns that the command is deprecated and prints the SSH command line in the chosen format
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [SSH local port, Hostname, Guest user]}
    contexts:
      cli:
        place: limactl
---

# Show how to reach an instance over SSH

## Trigger

The User runs `limactl show-ssh NAME`.

## Outcome

The User has an SSH command line for the instance.

## Edge cases

- `--format` chooses a command line, arguments, options or an ssh config.
