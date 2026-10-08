---
actors:
  - user
references:
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#startAction
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#loadOrCreateInstance
---

# Start an instance from a template

## Goal

Go from a template to a running instance in one command.

## Success criterion

A new instance created from the template is running and ready.
