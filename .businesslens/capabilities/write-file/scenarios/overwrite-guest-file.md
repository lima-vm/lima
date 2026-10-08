---
kind: alternative
routes:
  agent: MCP
steps:
  - text: The AI agent calls `write_file` with the path of an existing file and its new content
    kind: actor
    actor: ai-agent
    entities:
      - {entity: file, effect: reads, facts: [Path]}
    contexts:
      agent:
        place: mcp
  - text: The Product overwrites the file's content
    kind: product
    actor: ai-agent
    entities:
      - {entity: file, effect: changes, facts: [Contents, Metadata]}
    contexts:
      agent:
        place: mcp
---

# Overwrite a guest file

## Trigger

The AI agent replaces the whole content of an existing file.

## Outcome

The file holds the new content.

## Edge cases

- A read-only mount or the guest's file permissions can refuse the write.
