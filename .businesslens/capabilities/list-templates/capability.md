---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#filterHiddenTemplates
  - kind: code
    role: implementation
    target: pkg/templatestore/templatestore.go#Templates
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

# List templates

List the names of the templates an instance can be created from: those Lima ships and those the User keeps in `$LIMA_HOME/_templates`, the first location winning for a name. Names starting with `_` are not listed.
