---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks for a shell in a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product opens a login shell in the guest, in the host's current directory when a mount shares it, otherwise in the guest user's home
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Mounts, SSH local port, Guest user]}
    contexts:
      cli:
        place: limactl
---

# Open an interactive shell

## Trigger

The User runs `lima`, or `limactl shell NAME`.

## Outcome

The User works in a shell inside the instance.

## Edge cases

- `--workdir` chooses the directory, and fails if it does not exist in the guest.
- `--shell` chooses the shell; Windows guests accept only their own shells.
- `--preserve-env` passes the host's environment, except variables that describe the host itself, such as `PATH` and `HOME`.
- `--reconnect` drops a stale SSH connection before connecting.
