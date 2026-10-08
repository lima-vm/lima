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
    target: pkg/mcp/toolset/filesystem.go#ReadFile
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/toolset.go#TranslateHostPath
---

# Read a file

Read a guest file (`read_file`), up to its first 32 MiB; a larger file is cut off without notice.
