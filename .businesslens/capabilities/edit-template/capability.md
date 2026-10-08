---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/edit.go#editAction
  - kind: code
    role: implementation
    target: cmd/limactl/template.go
  - kind: code
    role: implementation
    target: pkg/limatmpl/locator.go
  - kind: code
    role: implementation
    target: pkg/limatmpl/embed.go
  - kind: code
    role: implementation
    target: pkg/templatestore/templatestore.go
---

# Edit a template

Change a template kept in a local YAML file, in the User's editor or with flags and `--set` expressions, saving it only when it validates.
