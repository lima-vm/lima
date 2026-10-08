---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to prune the download cache
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product removes the whole download cache
    kind: product
    actor: user
    entities:
      - {entity: cached-download, effect: removes}
    contexts:
      cli:
        place: limactl
---

# Prune every cached download

## Trigger

The User runs `limactl prune`.

## Outcome

No downloads are cached; instances download their images again when they need them.
