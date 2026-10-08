---
kind: primary
routes:
  agent: MCP
steps:
  - text: The AI agent asks to write content at a new absolute path
    kind: actor
    actor: ai-agent
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      agent:
        place: mcp
  - text: The Product creates the file and any missing parent directories with that content
    kind: product
    actor: ai-agent
    entities:
      - {entity: file, effect: creates, facts: [Path, Contents, Metadata]}
    contexts:
      agent:
        place: mcp
---

# Write a new guest file

## Trigger

The AI agent calls `write_file` for a path where nothing exists yet.

## Outcome

The file and its parent directories exist and hold the content.

## Edge cases

- A relative or empty path is refused.
