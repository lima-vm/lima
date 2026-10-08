---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks for the sudoers content on Linux or Windows
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with "sudoers command is only supported on macOS right now"
    kind: condition
    entities: []
    contexts:
      cli:
        place: limactl
---

# Refuse on hosts other than macOS

## Trigger

The host is not macOS.

## Outcome

Nothing is printed.
