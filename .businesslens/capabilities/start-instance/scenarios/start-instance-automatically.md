---
kind: primary
routes:
  cli: Command line
steps:
  - text: A login session begins, or the host boots, for an instance registered to start then
    kind: condition
    unattended: true
    entities:
      - {entity: instance, effect: reads, facts: [Automatic startup]}
    contexts:
      cli:
        place: limactl
  - text: The Product boots the virtual machine as for a start by hand
    kind: product
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
---

# Start an instance automatically

## Trigger

The host's service manager starts an instance registered for automatic startup.

## Outcome

The instance is running without the User asking, and is started again if its host agent exits unexpectedly while keep-alive is on.
