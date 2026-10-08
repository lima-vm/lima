---
domain: templates
references:
  - kind: code
    role: implementation
    target: pkg/limatmpl/locator.go#Read
  - kind: code
    role: implementation
    target: pkg/limatmpl/embed.go#Embed
  - kind: code
    role: implementation
    target: pkg/templatestore/templatestore.go#Templates
  - kind: doc
    role: context
    target: templates/README.md
  - kind: doc
    role: context
    target: website/content/en/docs/templates/_index.md
  - kind: code
    role: implementation
    target: cmd/limactl/template.go
---

# Template

A YAML description of an instance that Lima creates instances from, addressed by a locator: `template:NAME` for the templates Lima ships and those the User keeps in `$LIMA_HOME/_templates`, a local file, an `http(s)` URL, a `github:` path, a custom scheme a URL plugin resolves, standard input, or an image URL from which Lima makes a minimal template.

## Information kept

- **Name** — the name the template is listed and addressed under, or the one derived from its locator
- **Locator** — where the template is read from
- **Base templates** — the templates it builds on, merged into it when an instance is created
- **Settings** — the instance settings it declares: VM type, images, resources, mounts, port forwards, provisioning and the rest
- **Parameters** — the named values it declares for its scripts and rules
