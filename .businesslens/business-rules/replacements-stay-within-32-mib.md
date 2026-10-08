---
appliesTo:
  - type: capability
    id: replace-text
    contexts:
      - place: mcp
references:
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/filesystem.go#Replace
---

# A replacement reads and writes at most 32 MiB

`replace` refuses a file larger than 32 MiB, and a replacement whose result
would be larger.
