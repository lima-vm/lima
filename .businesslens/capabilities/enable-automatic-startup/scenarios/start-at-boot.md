---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks to register an instance to start at boot on macOS
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product installs a system service for the instance with sudo, running as the chosen user
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, facts: [Automatic startup]}
    contexts:
      cli:
        place: limactl
---

# Start an instance at boot

## Trigger

The User runs `limactl autostart enable NAME --condition boot` on macOS.

## Outcome

The instance starts whenever the host boots, before anyone logs in.

## Edge cases

- `--user` chooses the macOS user the instance runs as, `$USER` by default.
