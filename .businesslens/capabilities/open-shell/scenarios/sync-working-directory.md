---
kind: alternative
routes:
  cli: Command line
steps:
  - text: The User asks for a shell with the current directory synced into the guest
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product copies the host directory into the guest and opens the shell there
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Mounts, SSH local port]}
    contexts:
      cli:
        place: limactl
  - text: The User leaves the shell and accepts the changes made in the guest
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product copies the changed files back to the host directory and removes the guest copy
    kind: product
    actor: user
    entities:
      - {entity: file, effect: changes, facts: [Contents, Metadata]}
    contexts:
      cli:
        place: limactl
---

# Work on a synced copy of the current directory

## Trigger

The User runs `limactl shell --sync . NAME` on an instance without mounts.

## Outcome

The host directory holds the changes the User accepted.

## Edge cases

- The User may view the changed contents before accepting or rejecting them.
- Without a terminal the changes are copied back without asking; a failed command ends without copying back.
- The Product refuses `--sync` for instances with mounts, for WSL2 instances, together with `--workdir`, without rsync, or for a directory fewer than four levels deep.
