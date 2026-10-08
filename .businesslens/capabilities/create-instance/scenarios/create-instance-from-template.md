---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User runs `limactl create` with a template locator
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product reads the template and embeds its base templates, including the global base in `$LIMA_HOME/_config/base.yaml`
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator, Base templates, Settings, Parameters]}
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
  - text: The Product downloads the images the configuration names unless they are already cached, and checks their digests
    kind: product
    actor: user
    entities:
      - {entity: cached-download, effect: creates, facts: [URL, Data, Digest]}
    contexts:
      cli:
        place: limactl
  - text: The Product prepares the instance's disk from the image
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, facts: [Guest disk]}
    contexts:
      cli:
        place: limactl
  - text: The Product prints how to start the instance
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Create an instance from a template

## Trigger

The User runs `limactl create` with a template locator such as `template:docker`, or with none for the default template.

## Outcome

A stopped instance exists with the template's configuration and is ready to start.

## Edge cases

- A template read from standard input needs `--name`.
- An image URL instead of a template creates an instance from a minimal template that boots that image.
- `--set` expressions, `--param` values and flags such as `--cpus`, `--memory`, `--mount` or `--port-forward` change the configuration before it is created.
- An image that cannot be downloaded from any of its locations fails the command after the instance is created; starting it later downloads again.
