---
appliesTo:
  - type: entity
    id: instance
    effect: removes
permits:
  - actors:
      - user
    when:
      - {fact: Protection, is: 'Off'}
references:
  - kind: code
    role: implementation
    target: pkg/instance/delete.go#Delete
---

# Only an unprotected instance is deleted

Deleting an instance, with or without `--force`, is refused while it is
protected; the User removes the protection first.

## Rationale

Protection exists to prevent accidental removal of an instance someone relies
on.
