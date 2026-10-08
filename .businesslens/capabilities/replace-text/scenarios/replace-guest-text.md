---
kind: primary
routes:
  agent: MCP
steps:
  - text: The AI agent calls `replace` with a path, the old text and the new text
    kind: actor
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path]}
    contexts:
      agent:
        place: mcp
  - text: The Product replaces the expected occurrences and saves the file in place
    kind: product
    actor: ai-agent
    entities:
      - {entity: file, effect: changes, facts: [Contents]}
    contexts:
      agent:
        place: mcp
---

# Replace text in a guest file

## Trigger

The AI agent needs to change part of a file.

## Outcome

The file holds the new text.

## Edge cases

- The file keeps its permissions; if saving fails, the Product tries to restore the original content.
