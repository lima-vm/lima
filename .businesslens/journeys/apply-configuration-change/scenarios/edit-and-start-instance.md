---
kind: primary
result: achieved
routes:
  cli: Command line
steps:
  - text: The User asks to edit a stopped instance
    capability: edit-instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product validates the changed configuration and saves it
    capability: edit-instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Stopped, facts: [Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]}
    contexts:
      cli:
        place: limactl
  - text: The Product asks whether to start the instance now
    capability: edit-instance
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
---

# Edit an instance and start it

## Trigger

The User runs `limactl edit NAME` in a terminal, or with `--start`.

## Outcome

The instance is running with its new configuration.
