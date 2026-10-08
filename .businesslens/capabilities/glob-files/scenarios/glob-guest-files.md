---
kind: primary
routes:
  agent: MCP
steps:
  - text: The AI agent calls `glob` with a pattern
    kind: actor
    actor: ai-agent
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      agent:
        place: mcp
  - text: The Product returns the paths of the matching files
    kind: product
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path]}
    contexts:
      agent:
        place: mcp
---

# Find guest files by pattern

## Trigger

The AI agent needs to find files.

## Outcome

The AI agent has the matching paths, or an empty list.

## Edge cases

- The paths are not sorted by modification time.
