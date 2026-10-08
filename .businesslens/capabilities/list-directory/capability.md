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
    target: pkg/mcp/toolset/filesystem.go#ListDirectory
---

# List a directory

List the entries of a guest directory (`list_directory`), with their size, mode, modification time and whether each is a directory.
