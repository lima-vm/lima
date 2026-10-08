---
kind: primary
result: achieved
routes:
  cli: Command line
steps:
  - text: The User asks to rename a stopped instance
    capability: rename-instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product moves the instance to its new name and asks whether to start it
    capability: rename-instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Stopped, facts: [Name, Hostname, Directory, Protection]}
    contexts:
      cli:
        place: limactl
  - text: The User answers yes
    capability: start-instance
    kind: actor
    actor: user
    entities: []
    contexts:
      cli:
        place: limactl
  - text: The Product starts the instance and waits until it is ready
    capability: start-instance
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: changes, from: Stopped, to: Running, facts: [SSH local port, Guest disk]}
    contexts:
      cli:
        place: limactl
---

# Rename an instance and start it

## Trigger

The User runs `limactl rename OLD NEW` in a terminal and answers yes, or with `--start`.

## Outcome

The instance is running under its new name.
