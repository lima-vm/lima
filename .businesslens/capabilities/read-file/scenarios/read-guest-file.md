---
kind: primary
routes:
  agent: MCP
steps:
  - text: The AI agent calls `read_file` with an absolute path
    kind: actor
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path]}
    contexts:
      agent:
        place: mcp
  - text: The Product returns the file's content
    kind: product
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path, Contents]}
    contexts:
      agent:
        place: mcp
---

# Read a guest file

## Trigger

The AI agent needs a file's content.

## Outcome

The AI agent has the file's content.

## Edge cases

- A relative or empty path is refused.
- A file the guest user cannot read, or a lost connection to the guest, is reported as an error.
