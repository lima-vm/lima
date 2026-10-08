---
kind: edge
routes:
  cli: Command line
steps:
  - text: The User asks to prune but keep what is referred to
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: An instance is being set up, and the Product refuses with "cannot safely prune while instance is being used"
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Refuse to prune while an instance is being set up

## Trigger

An instance directory has no configuration yet, as while an instance is being created or cloned, while the User prunes with `--keep-referred`.

## Outcome

Nothing is removed.
