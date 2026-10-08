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
    target: cmd/limactl/template.go#templateValidateAction
  - kind: code
    role: implementation
    target: pkg/limayaml/validate.go
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

# Validate a template

Check that templates are valid Lima configurations once their base templates and Lima's defaults are merged in, optionally printing them with every default filled in. `limactl validate` is the same command.
