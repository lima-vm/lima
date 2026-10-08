---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to check the installed sudoers content
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product compares it with what it would generate and refuses when they differ, with a hint to regenerate it
    kind: product
    actor: user
    entities:
      - {entity: network, effect: reads, facts: [Name, Mode]}
    contexts:
      cli:
        place: limactl
---

# Check the installed sudoers content

## Trigger

The User runs `limactl sudoers --check`, optionally naming the path to check.

## Outcome

The User knows whether the installed content is current.

## Edge cases

- With `--block-device`, the whole generated content is compared, so entries kept by hand or for other users are reported as out of sync.
- The check installs nothing.
