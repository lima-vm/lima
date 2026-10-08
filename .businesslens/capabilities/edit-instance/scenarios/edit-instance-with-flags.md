---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to edit a stopped instance with flags such as `--cpus`, `--mount` or `--set`
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product applies the changes, validates the configuration and saves it to the instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Stopped, facts: [Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode]}
    contexts:
      cli:
        place: limactl
---

# Edit an instance with flags

## Trigger

The User runs `limactl edit NAME --cpus 4`, or with another edit flag or `--set` expression.

## Outcome

The instance keeps the changed configuration.

## Edge cases

- `--mount` adds mounts, `--mount-only` replaces them, `--mount-none` removes them all and `--mount-writable` makes them all writable.
- `--port-forward`, `--network` and `--block-device` add port forwards, networks and host block devices.
