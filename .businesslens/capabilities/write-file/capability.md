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
    target: pkg/mcp/toolset/filesystem.go#WriteFile
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/toolset.go#TranslateHostPath
---

# Write a file

Create or overwrite a guest file (`write_file`), creating its parent directories.
