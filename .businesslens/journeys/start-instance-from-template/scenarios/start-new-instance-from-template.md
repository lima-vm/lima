---
kind: primary
result: achieved
routes:
  cli: Command line
steps:
  - text: The User asks to start from a template locator
    capability: create-instance
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product creates the stopped instance from the template
    capability: create-instance
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator, Base templates, Settings, Parameters]}
      - {entity: instance, effect: creates, to: Stopped, facts: [Name, Hostname, Directory, VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode, Guest disk, Protection, Automatic startup, Lima version]}
    contexts:
      cli:
        place: limactl
  - text: The Product starts the new instance and waits until it is ready
    capability: start-instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
  - text: The Product reports READY with how to open a shell
    capability: start-instance
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name, Message]}
    contexts:
      cli:
        place: limactl
---

# Start a new instance from a template

## Trigger

The User runs `limactl start template:docker`, or `limactl start NAME` for a name with no instance.

## Outcome

The new instance is running.
