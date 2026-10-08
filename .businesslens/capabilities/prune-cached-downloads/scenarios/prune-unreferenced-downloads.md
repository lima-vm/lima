---
kind: alternative
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
  - text: The Product removes the cached downloads no instance or template names, and partial downloads
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Images]}
      - {entity: template, effect: reads, facts: [Settings]}
      - {entity: cached-download, effect: removes}
    contexts:
      cli:
        place: limactl
---

# Keep the downloads instances and templates refer to

## Trigger

The User runs `limactl prune --keep-referred`.

## Outcome

Only the downloads instances and templates still need remain cached.

## Edge cases

- Instances whose configuration has errors are skipped with a warning.
