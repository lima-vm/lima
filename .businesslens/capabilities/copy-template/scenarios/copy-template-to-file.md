---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to copy a template to a destination
    kind: actor
    actor: user
    entities:
      - {entity: template, as: source, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product writes the template to the destination
    kind: product
    actor: user
    entities:
      - {entity: template, as: source, effect: reads, facts: [Locator, Base templates, Settings, Parameters]}
      - {entity: template, as: copy, effect: creates, facts: [Name, Locator, Base templates, Settings, Parameters]}
    contexts:
      cli:
        place: limactl
---

# Copy a template to a file

## Trigger

The User runs `limactl template copy template:docker ./my-docker.yaml`.

## Outcome

The destination holds a template the User can create instances from.

## Edge cases

- `--embed` merges the base templates into the copy, `--fill` fills in every default, and `--verbatim` copies it unchanged; `--verbatim` cannot be combined with the others.
- Without a destination, or with `-`, the template is written to standard output.
- An existing destination is overwritten.
- A locator Lima cannot interpret is refused.
