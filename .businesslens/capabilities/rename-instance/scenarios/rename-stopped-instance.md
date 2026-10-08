---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to rename an instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product moves the instance to its new name and drops its protection
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Stopped, facts: [Name, Hostname, Directory, Protection]}
    contexts:
      cli:
        place: limactl
---

# Rename a stopped instance

## Trigger

The User runs `limactl rename OLD NEW`.

## Outcome

The instance exists only under its new name.

## Edge cases

- `--start` starts the renamed instance afterwards; with a terminal attached the User is asked whether to start it.
- A new name equal to the old one, already in use, or too long is refused.
