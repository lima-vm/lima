---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to edit a template by its path
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product opens the template in the User's editor
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Name, Base templates, Settings, Parameters]}
    contexts:
      cli:
        place: limactl::configuration-editor
  - text: The User changes the configuration and saves it
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl::configuration-editor
  - text: The Product validates the template and saves it
    kind: product
    actor: user
    entities:
      - {entity: template, effect: changes, facts: [Base templates, Settings, Parameters]}
    contexts:
      cli:
        place: limactl::configuration-editor
---

# Edit a template in the editor

## Trigger

The User runs `limactl edit FILE.yaml` with a terminal attached.

## Outcome

The template keeps the changes.

## Edge cases

- Saving an empty buffer, or saving without changes, leaves the template unchanged.
- A template that does not validate, or that shrinks its disk, is refused and kept as `lima.REJECTED.yaml`.
- The Product does not offer to start anything after editing a template.
