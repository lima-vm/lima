---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to copy files, naming guest paths as `INSTANCE:PATH`
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
      - {entity: file, as: source, effect: reads, facts: [Path]}
    contexts:
      cli:
        place: limactl
  - text: The Product copies the files over SSH and reports when it is done
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [SSH local port]}
      - {entity: file, as: source, effect: reads, facts: [Path, Contents]}
      - {entity: file, as: copy, effect: creates, facts: [Path, Contents, Metadata]}
    contexts:
      cli:
        place: limactl
---

# Copy files between the host and an instance

## Trigger

The User runs `limactl copy SOURCE… TARGET`, naming guest paths as `INSTANCE:PATH`.

## Outcome

The files exist at the target.

## Edge cases

- `-r` copies directories recursively, behaving like `cp -r`.
- An existing file at the target is overwritten.
- `--backend` chooses scp or rsync and fails when the chosen tool is missing on the host or a guest.
- Copying between two instances goes through the host, and with scp needs OpenSSH 8.0 or newer.
