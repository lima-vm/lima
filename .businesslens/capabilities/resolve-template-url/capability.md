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

# Resolve a template URL

Turn a custom template locator — a `github:` path or a scheme a `limactl-url-SCHEME` plugin handles — into the file or `https` URL Lima would read.
