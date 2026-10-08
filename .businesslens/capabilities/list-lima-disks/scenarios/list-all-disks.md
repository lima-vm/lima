---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to list Lima disks
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl::disk-list
  - text: The User reads each Lima disk's name, size, format, directory and holder
    kind: actor
    actor: user
    entities:
      - {entity: lima-disk, effect: reads, facts: [Name, Size, Format, Directory, In use by]}
    contexts:
      cli:
        place: limactl::disk-list
---

# List disks

## Trigger

The User runs `limactl disk list`.

## Outcome

The User sees the Lima disks.

## Edge cases

- `--json` prints JSON.
- With no disks the Product warns and lists nothing; a disk that cannot be inspected is skipped with an error message.
