---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to clone an instance under a new name
    kind: actor
    actor: user
    entities:
      - {entity: instance, as: original, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product copies the instance's directory into a new stopped instance, applying any edit flags given
    kind: product
    actor: user
    entities:
      - {entity: instance, as: original, effect: reads, facts: [Name]}
      - {entity: instance, as: copy, effect: creates, to: Stopped, facts: [Name, Hostname, Directory, VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode, Guest disk, Protection, Lima version]}
    contexts:
      cli:
        place: limactl
---

# Clone a stopped instance

## Trigger

The User runs `limactl clone OLD NEW`.

## Outcome

A new stopped instance exists with the original's configuration and disk.

## Edge cases

- `--start` starts the new instance afterwards; with a terminal attached the User is asked whether to start it.
- A new name equal to the old one, already in use, or too long is refused.
- Edit flags that leave the configuration invalid are refused and kept as `lima.REJECTED.yaml`.
