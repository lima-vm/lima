---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/list.go#listAction
  - kind: code
    role: implementation
    target: pkg/store/instance.go#PrintInstances
  - kind: code
    role: implementation
    target: pkg/store/instance.go#Inspect
  - kind: code
    role: implementation
    target: pkg/store/instance.go#inspectStatusWithPIDFiles
---

# List instances

Show every instance, or those named or matching a filter, with its status and resources, as a table or as JSON, YAML or a Go template for scripts.
