---
kind: edge
routes:
  cli: Command line
steps:
  - text: The User asks to start a stopped instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product boots the virtual machine, but its guest agent does not answer
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
  - text: 'The Product reports DEGRADED: the instance runs, but mounts and port forwarding may not work'
    kind: condition
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
---

# Start an instance whose guest agent does not answer

## Trigger

A requirement other than SSH fails while the instance starts, for example the guest agent never answers.

## Outcome

The instance runs, and the User is told it is degraded and where its logs are.
