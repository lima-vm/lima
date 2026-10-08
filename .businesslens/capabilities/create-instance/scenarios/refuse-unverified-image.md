---
kind: edge
routes:
  cli: Command line
steps:
  - text: The User runs `limactl create` with a template whose image declares a digest
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product creates the stopped instance with that configuration
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: creates, to: Stopped, facts: [Name, Hostname, Directory, VM type, Guest OS, Architecture, Images, CPUs, Memory, Disk size, Mounts, Port forwards, Networks, Additional disks, Host block devices, Guest user, Provisioning, Probes, Containerd, Environment, Parameters, Copy to host, Message, Plain mode, Guest disk, Protection, Automatic startup, Lima version]}
    contexts:
      cli:
        place: limactl
  - text: The downloaded image does not match the declared digest, and the Product fails with "expected digest"
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Images]}
    contexts:
      cli:
        place: limactl
---

# Refuse an image that does not match its digest

## Trigger

An image named by the template has a digest that the downloaded content does not match.

## Outcome

The command fails. The stopped instance stays without a prepared disk until it is started or deleted.
