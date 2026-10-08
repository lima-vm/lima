---
availability:
  - place: limactl
domain: templates
references:
  - kind: code
    role: implementation
    target: cmd/limactl/template.go
  - kind: code
    role: implementation
    target: cmd/limactl/template.go#templateCopyAction
  - kind: code
    role: implementation
    target: pkg/limatmpl/embed.go#Embed
  - kind: code
    role: implementation
    target: pkg/limatmpl/locator.go
  - kind: code
    role: implementation
    target: pkg/limatmpl/embed.go
  - kind: code
    role: implementation
    target: pkg/templatestore/templatestore.go
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#filterHiddenTemplates
---

# Copy a template

Copy a template to a file or to standard output, optionally with its base templates and scripts embedded or its defaults filled in, so it can be kept and changed as the User's own. The `template` commands are experimental.
