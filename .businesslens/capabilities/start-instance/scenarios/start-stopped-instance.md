---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to start a stopped instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product attaches the instance's Lima disks, holding each for this instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Additional disks]}
      - {entity: lima-disk, effect: changes, from: Available, to: In use, facts: [In use by]}
    contexts:
      cli:
        place: limactl
  - text: The Product boots the virtual machine, shares the mounts, runs the provisioning and waits until the readiness requirements and probes pass
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
  - text: The Product reports READY with how to open a shell, followed by the instance's message
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name, Message]}
    contexts:
      cli:
        place: limactl
---

# Start a stopped instance

## Trigger

The User runs `limactl start NAME`, or `limactl start` for the default instance.

## Outcome

The instance is running and ready, its mounts shared and its ports being forwarded.

## Edge cases

- Starting an instance that is already running only reports that it is running.
- `--set` expressions and edit flags given to start change the configuration of an existing instance first.
- `--timeout` bounds the wait for readiness, ten minutes by default and thirty for a Windows guest's first boot.
- In plain mode the instance starts without mounts, dynamic port forwarding, containerd or the guest agent.
- Files the configuration copies to the host are copied once the guest is ready.
