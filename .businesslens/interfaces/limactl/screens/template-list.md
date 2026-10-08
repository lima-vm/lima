---
entities:
  - entity: template
    shows: [Name]
entryPoints:
  - limactl: limactl create --list-templates
references:
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#filterHiddenTemplates
---

# Template list

The names of the templates that can be named as `template:NAME` when creating
or starting an instance.
