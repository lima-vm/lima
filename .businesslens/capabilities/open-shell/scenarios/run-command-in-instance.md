---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to run a command in a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product runs the command in the guest's login shell and returns its output and exit status
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Mounts, SSH local port]}
    contexts:
      cli:
        place: limactl
---

# Run one command

## Trigger

The User runs `lima uname -a`, or `nerdctl.lima run …` and the other wrappers.

## Outcome

The command has run inside the instance, and its exit status is the command's.
