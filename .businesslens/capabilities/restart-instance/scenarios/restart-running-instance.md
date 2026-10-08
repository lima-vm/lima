---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to restart a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product stops the instance gracefully
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Running, to: Stopped, facts: [Forwarded ports]}
    contexts:
      cli:
        place: limactl
  - text: The Product starts the instance again and waits until it is ready
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
---

# Restart a running instance

## Trigger

The User runs `limactl restart NAME`.

## Outcome

The instance is running again.

## Edge cases

- `--force` stops the instance by force before starting it.
- Edit flags given to restart change the configuration before it starts again; a configuration that does not validate is kept as `lima.REJECTED.yaml`.
