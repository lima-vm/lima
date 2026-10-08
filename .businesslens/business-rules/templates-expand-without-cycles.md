---
appliesTo:
  - type: entity
    id: template
    facts: [Base templates]
references:
  - kind: code
    role: implementation
    target: pkg/limatmpl/embed.go#Embed
---

# A template's base templates are embedded without cycles

Base templates are embedded recursively up to a size limit. A base template that
is already included, or a base declared with a digest, is refused.
