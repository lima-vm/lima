---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to create a Lima disk with a name and size
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product creates the Lima disk, ready to be attached
    kind: product
    actor: user
    entities:
      - {entity: lima-disk, effect: creates, to: Available, facts: [Name, Size, Format, Directory]}
    contexts:
      cli:
        place: limactl
---

# Create a disk

## Trigger

The User runs `limactl disk create NAME --size SIZE`.

## Outcome

An available Lima disk exists that instances can attach.

## Edge cases

- A name already in use is refused with "disk already exists".
- A format other than `qcow2` or `raw` is refused.
