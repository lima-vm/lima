---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks for the names that `limactl create` accepts
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl::template-list
  - text: The Product lists the template names, leaving out hidden ones
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl::template-list
---

# List the templates to create from

## Trigger

The User runs `limactl create --list-templates`, or `limactl start --list-templates`.

## Outcome

The User knows which `template:NAME` locators exist.

## Edge cases

- Missing template directories are skipped.
