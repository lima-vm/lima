---
availability:
  - place: mcp
references:
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/filesystem.go
  - kind: code
    role: implementation
    target: pkg/mcp/msi/filesystem.go
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/filesystem.go#Replace
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/toolset.go#TranslateHostPath
---

# Replace text in a file

Replace an exact string in a guest file (`replace`), a given number of times — once by default.
