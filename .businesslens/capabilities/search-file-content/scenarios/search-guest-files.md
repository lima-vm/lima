---
kind: primary
routes:
  agent: MCP
steps:
  - text: The AI agent calls `search_file_content` with a pattern and a directory
    kind: actor
    actor: ai-agent
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
      - {entity: file, effect: reads, facts: [Path]}
    contexts:
      agent:
        place: mcp
  - text: The Product runs the search in the guest and returns the matching lines with their files and line numbers
    kind: product
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path, Contents]}
    contexts:
      agent:
        place: mcp
---

# Search guest files

## Trigger

The AI agent needs to find text in files.

## Outcome

The AI agent has the matching lines.

## Edge cases

- Only the search's output is returned: a failing search can return nothing rather than an error.
