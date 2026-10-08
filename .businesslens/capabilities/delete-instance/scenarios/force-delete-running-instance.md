---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to delete a running instance by force
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product stops the instance by force and releases its Lima disks
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Running, to: Stopped, facts: [Forwarded ports]}
      - {entity: lima-disk, effect: changes, from: In use, to: Available, facts: [In use by]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes the instance, its directory and its automatic-startup registration
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: removes, from: Stopped}
      - {entity: snapshot, effect: removes, with: instance}
      - {entity: instance-event, effect: removes, with: instance}
    contexts:
      cli:
        place: limactl
---

# Delete a running instance by force

## Trigger

The User runs `limactl delete --force NAME` on a running instance.

## Outcome

The instance no longer exists.
