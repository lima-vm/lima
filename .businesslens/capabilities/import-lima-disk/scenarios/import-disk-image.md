---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to import a disk image under a name
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product copies the image in as an available Lima disk
    kind: product
    actor: user
    entities:
      - {entity: lima-disk, effect: creates, to: Available, facts: [Name, Size, Format, Directory]}
    contexts:
      cli:
        place: limactl
---

# Import a disk image

## Trigger

The User runs `limactl disk import NAME FILE`.

## Outcome

An available Lima disk with the image's content exists.

## Edge cases

- A name already in use, or an image in another format, is refused.
- A copy that fails still ends the command successfully, leaving an empty disk directory.
