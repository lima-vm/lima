---
kind: primary
routes:
  agent: MCP
steps:
  - text: The AI agent calls `list_directory` with an absolute path
    kind: actor
    actor: ai-agent
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      agent:
        place: mcp
  - text: The Product returns the name and metadata of each file in the directory
    kind: product
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path, Metadata]}
    contexts:
      agent:
        place: mcp
---

# List a guest directory

## Trigger

The AI agent needs to see what a directory holds.

## Outcome

The AI agent has the directory's entries.

## Edge cases

- A relative or empty path is refused.
