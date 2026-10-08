---
references:
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/filesystem.go
  - kind: code
    role: implementation
    target: pkg/copytool/copytool.go#New
---

# File

A file or directory, addressed by its path, that the User copies between the host and an instance or that an AI agent reads, writes and searches inside an instance. A guest path inside a mounted host directory is the host's own file.

## Information kept

- **Path** — where it is: an absolute guest path, or `INSTANCE:PATH` or a host path for a copy
- **Contents** — what it holds
- **Metadata** — its size, permissions, modification time and whether it is a directory
