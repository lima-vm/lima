---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to register an instance to start at login
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product registers the instance with launchd or systemd
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, facts: [Automatic startup]}
    contexts:
      cli:
        place: limactl
---

# Start an instance at login

## Trigger

The User runs `limactl autostart enable NAME`.

## Outcome

The instance starts whenever the User logs in.

## Edge cases

- `--keep-alive=false` registers it without restarting it when it exits.
