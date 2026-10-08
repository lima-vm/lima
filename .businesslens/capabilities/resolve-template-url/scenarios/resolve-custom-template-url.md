---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to resolve a custom template locator
    kind: actor
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
  - text: The Product prints the URL the locator resolves to
    kind: product
    actor: user
    entities:
      - {entity: template, effect: reads, facts: [Locator]}
    contexts:
      cli:
        place: limactl
---

# Resolve a custom template locator

## Trigger

The User runs `limactl template url github:lima-vm/lima/templates/default`.

## Outcome

The User sees the regular file or `https` URL the locator stands for.

## Edge cases

- A plugin that fails, or a locator that keeps resolving to another custom locator, ends the command with an error.
- The URL is printed without checking that it holds a valid template.
