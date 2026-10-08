---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to factory reset an instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product erases what the guest wrote, its logs and its connection data, keeps the configuration and regenerates the cloud-init data
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Stopped, facts: [Guest disk, SSH local port]}
      - {entity: instance-event, effect: removes}
    contexts:
      cli:
        place: limactl
---

# Factory reset an instance

## Trigger

The User runs `limactl factory-reset NAME`.

## Outcome

The stopped instance keeps its configuration and boots a fresh guest at its next start.

## Edge cases

- A running instance is stopped by force first.
- A name with no instance is reported and nothing else happens.
- Files the reset cannot remove, such as non-empty directories, are left in place and the error is logged; the command still reports that the instance was reset.
