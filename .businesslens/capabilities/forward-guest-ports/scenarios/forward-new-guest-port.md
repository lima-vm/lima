---
kind: primary
routes:
  cli: Command line
steps:
  - text: A program in a running instance starts listening on a port
    kind: condition
    unattended: true
    entities:
      - {entity: instance, effect: reads, facts: [Port forwards]}
    contexts:
      cli:
        place: limactl
  - text: The Product forwards the matching host address and port to it, by the first matching rule or the localhost default
    kind: product
    entities:
      - {entity: instance, effect: changes, facts: [Forwarded ports]}
    contexts:
      cli:
        place: limactl
---

# Forward a port a guest program opens

## Trigger

The guest agent reports a new listening port.

## Outcome

Host programs reach the guest program on the forwarded host port.

## Edge cases

- A rule marked ignore leaves the port unforwarded.
- Kubernetes NodePort and LoadBalancer services in the guest are forwarded the same way.
- A host port that is busy, or a forward that fails, is reported as a warning.
