---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks for diagnostic information
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product prints it as JSON, filtered by any `--yq` expression
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Name, Settings]}
    contexts:
      cli:
        place: limactl
---

# Show diagnostic information

## Trigger

The User runs `limactl info`.

## Outcome

The User sees how Lima is set up on this host.

## Edge cases

- No registered VM driver, or a default template that cannot be read, ends the command with an error.
