---
kind: validation
routes:
  agent: MCP
steps:
  - text: The AI agent calls `replace` on a file
    kind: actor
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path]}
    contexts:
      agent:
        place: mcp
  - text: The old text is empty, equals the new text, is not found or occurs a different number of times than expected, and the Product refuses
    kind: condition
    entities:
      - {entity: file, effect: reads, facts: [Contents]}
    contexts:
      agent:
        place: mcp
---

# Refuse a replacement that does not match

## Trigger

The replacement does not match the file as asked.

## Outcome

The file is unchanged.

## Edge cases

- A file, or a result, larger than 32 MiB is refused.
