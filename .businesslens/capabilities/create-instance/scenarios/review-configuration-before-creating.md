---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User runs `limactl create` with a template locator in a terminal
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product offers to proceed with the configuration, open it in the editor, choose another template or exit
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Name, Base templates, Settings, Parameters]}
    contexts:
      cli:
        place: limactl::configuration-editor
  - text: The User opens the configuration in the editor, changes it and saves it
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl::configuration-editor
  - text: The Product validates the saved configuration and creates the stopped instance with it
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: creates, to: Stopped, facts: [Name, Hostname, Directory, VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode, Guest disk, Protection, Automatic startup, Lima version]}
    contexts:
      cli:
        place: limactl::configuration-editor
---

# Review the configuration before creating

## Trigger

The User runs `limactl create` with a terminal attached.

## Outcome

A stopped instance exists with the configuration the User saved.

## Edge cases

- Saving an empty buffer, or choosing to exit, creates nothing.
- Choosing another template offers the shipped templates by name.
- A configuration that does not validate is refused and kept as `lima.REJECTED.yaml` in the current directory.
