---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to stop a broken instance by force
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name, Errors]}
    contexts:
      cli:
        place: limactl
  - text: The Product kills the virtual machine and host agent processes and removes the process IDs and sockets they left behind
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Broken, to: Stopped, facts: [Forwarded ports, Errors]}
    contexts:
      cli:
        place: limactl
  - text: The Product releases the instance's Lima disks
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Additional disks]}
      - {entity: lima-disk, effect: changes, from: In use, to: Available, facts: [In use by]}
    contexts:
      cli:
        place: limactl
---

# Force an instance to stop

## Trigger

The User runs `limactl stop --force NAME`, typically on a broken instance.

## Outcome

The instance is stopped, whatever state it was in.

## Edge cases

- A running instance is stopped by force the same way.
