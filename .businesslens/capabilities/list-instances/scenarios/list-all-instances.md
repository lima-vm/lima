---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to list instances
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl::instance-list
  - text: The Product inspects every instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name, Hostname, SSH local port, VM type, Guest OS, Architecture, CPUs, Memory, Disk size, Directory, Protection, Automatic startup, Lima version, Networks, Additional disks, Parameters, Message, Errors]}
    contexts:
      cli:
        place: limactl::instance-list
  - text: The User reads each instance's name, status, SSH address, VM type, architecture, resources and directory
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name, Hostname, SSH local port, VM type, Guest OS, Architecture, CPUs, Memory, Disk size, Directory, Protection, Automatic startup, Lima version, Networks, Additional disks, Parameters, Message, Errors]}
    contexts:
      cli:
        place: limactl::instance-list
---

# List all instances

## Trigger

The User runs `limactl list`.

## Outcome

The User sees every instance and its status.

## Edge cases

- With no instances the Product suggests creating one.
- Names given that match no instance end the command with an error after listing the others.
- `--filter` and `--yq` select and reshape the instances; `--quiet` prints names only, and `--all-fields` shows every field.
