---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to query a template with a yq expression
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product embeds the base templates, fills in the defaults and prints what the expression selects
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator, Base templates, Settings, Parameters]}
    contexts:
      cli:
        place: limactl
---

# Query a template with an expression

## Trigger

The User runs `limactl template yq template:default '.images[].location'`.

## Outcome

The User sees the values the expression selects.

## Edge cases

- A template that cannot be read, or an expression that is not valid, ends the command with an error.
