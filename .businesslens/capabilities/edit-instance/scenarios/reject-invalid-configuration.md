---
kind: validation
routes:
  cli: Command line
steps:
  - text: The User asks to edit a stopped instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The User saves a configuration that does not validate, such as one with a smaller disk
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl::configuration-editor
  - text: The Product refuses it and keeps it as `lima.REJECTED.yaml` in the current directory
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Disk size]}
    contexts:
      cli:
        place: limactl::configuration-editor
---

# Reject a configuration that does not validate

## Trigger

The saved configuration fails validation, for example because it shrinks the guest's disk.

## Outcome

The instance keeps its previous configuration.
