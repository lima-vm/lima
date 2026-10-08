---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to stop a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product shuts the guest down and waits for the virtual machine and host agent to exit
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Running, to: Stopped, facts: [Forwarded ports]}
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

# Stop a running instance

## Trigger

The User runs `limactl stop NAME`, or `limactl stop` for the default instance.

## Outcome

The instance is stopped and its Lima disks are free for other instances.

## Edge cases

- The Product gives up when the instance has not shut down after about three minutes.
- An instance registered for automatic startup is stopped through the host's service manager.
- Several names stop each instance in turn; errors are reported for those that fail.
