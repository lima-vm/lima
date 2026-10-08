---
availability:
  - place: mcp
references:
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/shell.go#RunShellCommand
  - kind: code
    role: implementation
    target: pkg/mcp/msi/shell.go
---

# Run a shell command

Run a command in the guest in a given directory (`run_shell_command`) and return its output and exit code.
