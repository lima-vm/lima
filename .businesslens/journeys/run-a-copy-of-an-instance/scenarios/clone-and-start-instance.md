---
kind: primary
result: achieved
routes:
  cli: Command line
steps:
  - text: The User asks to clone a stopped instance
    capability: clone-instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, as: original, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product copies the instance into a new stopped instance and asks whether to start it
    capability: clone-instance
    kind: product
    actor: user
    entities:
      - {entity: instance, as: original, effect: reads, facts: [Name]}
      - {entity: instance, as: copy, effect: creates, to: Stopped, facts: [Name, Hostname, Directory, VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode, Guest disk, Protection, Lima version]}
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
  - text: The Product starts the copy and waits until it is ready
    capability: start-instance
    kind: product
    actor: user
    entities:
      - {entity: instance, as: copy, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
---

# Clone an instance and start the copy

## Trigger

The User runs `limactl clone OLD NEW` in a terminal and answers yes, or with `--start`.

## Outcome

The copy is running.
