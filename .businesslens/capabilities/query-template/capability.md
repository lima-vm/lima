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

# Query a template

Evaluate a yq expression against a template after its base templates are embedded and its defaults filled in, and print the result.
