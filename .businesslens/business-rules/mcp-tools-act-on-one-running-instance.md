---
appliesTo:
  - type: context
    context:
      place: mcp
references:
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/toolset.go#RegisterInstance
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/toolset.go#TranslateHostPath
---

# The MCP tools act on one running instance

The MCP server serves one instance, which must be running and free of errors
when it starts, and its tools take absolute paths only. They act as the guest
user, so the guest's permissions and read-only mounts limit what they change.
