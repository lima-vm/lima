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
    target: pkg/mcp/toolset/filesystem.go#SearchFileContent
---

# Search file content

Search guest files under a directory for a regular expression (`search_file_content`) with `git grep`, optionally limited to an include pattern.
