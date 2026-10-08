---
actors:
  - user
references:
  - kind: code
    role: implementation
    target: cmd/limactl/shell.go#shellAction
---

# Open a shell in a stopped instance

## Goal

Get a shell in an instance that is not running yet.

## Success criterion

The User is in a shell inside the instance, which is now running.
