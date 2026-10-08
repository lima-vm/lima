---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User runs `limactl create` with a template and settings that do not validate together
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator, Settings]}
    contexts:
      cli:
        place: limactl
  - text: The Product refuses with the validation errors
    kind: condition
    entities:
      - {entity: template, effect: reads, facts: [Settings]}
    contexts:
      cli:
        place: limactl
---

# Refuse a configuration that does not validate

## Trigger

The template, with the settings given on the command line, is not a valid configuration.

## Outcome

Nothing is created.

## Edge cases

- The VM type is validated too: a setting the chosen driver does not support is refused.
