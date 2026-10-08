---
kind: edge
routes:
  cli: Command line
steps:
  - text: The User asks to start a broken instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name, Errors]}
    contexts:
      cli:
        place: limactl
  - text: The Product clears the stale host agent leftovers, or stops the orphaned virtual machine by force, and starts the instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Broken, to: Running, facts: [SSH local port, Guest disk, Errors]}
    contexts:
      cli:
        place: limactl
---

# Recover an instance left broken by an unclean shutdown

## Trigger

The User starts an instance whose host agent left stale files behind, or whose virtual machine kept running after its host agent exited.

## Outcome

The leftovers are cleaned up and the instance starts afresh; an instance with other errors is not started.

## Edge cases

- Only a host agent socket that refuses connections counts as stale; other errors end the start with "errors inspecting instance".
- Stale host agent files are removed without signalling the recorded process, whose ID may since belong to another program.
