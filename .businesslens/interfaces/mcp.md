---
type: agent
actors:
  - ai-agent
entryPoints:
  - agent: limactl mcp serve
references:
  - kind: code
    role: implementation
    target: cmd/limactl-mcp/main.go#mcpServeAction
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/toolset.go#RegisterServer
  - kind: doc
    role: context
    target: website/content/en/docs/config/ai/outside/_index.md
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/toolset.go#RegisterInstance
---

# MCP server

The experimental Model Context Protocol server, served over standard input and
output by the `limactl mcp` plugin, through which an AI agent works inside one
instance — `default` unless another is named when it is started. Its tools take
absolute paths, which are used unchanged inside the guest, so they reach the
User's files where the instance mounts them at the same path.
