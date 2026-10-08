---
kind: primary
result: achieved
routes:
  cli: Command line
steps:
  - text: The User asks for a shell in a stopped instance
    capability: open-shell
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product asks whether to start the instance now
    capability: open-shell
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The User answers yes
    capability: start-instance
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product starts the instance and waits until it is ready
    capability: start-instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
  - text: The Product opens the shell in the guest
    capability: open-shell
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Mounts, SSH local port]}
    contexts:
      cli:
        place: limactl
---

# Start the instance to open a shell

## Trigger

The User runs `lima` or `limactl shell NAME` in a terminal, or with `--start`, while the instance is stopped.

## Outcome

The User is in a shell in the running instance.
