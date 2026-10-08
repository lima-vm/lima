---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to edit a stopped instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product opens the instance's configuration in the User's editor
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]}
    contexts:
      cli:
        place: limactl::configuration-editor
  - text: The User changes the configuration and saves it
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl::configuration-editor
  - text: The Product validates the configuration and saves it to the instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Stopped, facts: [Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]}
    contexts:
      cli:
        place: limactl::configuration-editor
---

# Edit an instance in the editor

## Trigger

The User runs `limactl edit NAME` with a terminal attached.

## Outcome

The instance keeps the changed configuration.

## Edge cases

- Saving an empty buffer, or saving without changes, leaves the instance unchanged.
- Without `$VISUAL` or `$EDITOR`, Lima tries `editor`, `vim`, `vi` and `emacs` and refuses when none is found.
