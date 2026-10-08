---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to validate one or more templates
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product reports each valid template as OK and stops at the first one that is not, with its errors
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator, Base templates, Settings, Parameters]}
    contexts:
      cli:
        place: limactl
---

# Validate templates

## Trigger

The User runs `limactl template validate FILE.yaml…` or `limactl validate FILE.yaml…`.

## Outcome

The User knows whether the templates are valid.

## Edge cases

- `--fill` prints each valid template with its defaults filled in.
