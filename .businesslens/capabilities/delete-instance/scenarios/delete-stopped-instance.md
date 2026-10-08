---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks to delete a stopped instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product removes the instance, its directory and its automatic-startup registration
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: removes, from: Stopped}
      - {entity: snapshot, effect: removes, with: instance}
      - {entity: instance-event, effect: removes, with: instance}
    contexts:
      cli:
        place: limactl
---

# Delete a stopped instance

## Trigger

The User runs `limactl delete NAME`.

## Outcome

The instance no longer exists.

## Edge cases

- A name with no instance is ignored with a warning.
- A directory whose configuration cannot be read is not deleted, in case an instance is being cloned into it.
- A failure to remove the automatic-startup registration is reported as a warning after the instance is deleted.
