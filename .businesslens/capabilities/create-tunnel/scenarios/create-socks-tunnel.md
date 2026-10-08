---
kind: primary
routes:
  cli: Command line
steps:
  - text: The User asks for a tunnel to a running instance
    kind: actor
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [Name]}
    contexts:
      cli:
        place: limactl
  - text: The Product starts a SOCKS proxy on a host port and prints how to point programs at it
    kind: product
    actor: user
    entities:
      - {entity: instance, effect: reads, facts: [SSH local port, Hostname]}
    contexts:
      cli:
        place: limactl
---

# Create a SOCKS tunnel

## Trigger

The User runs `limactl tunnel NAME`.

## Outcome

Programs on the host can reach the instance through the proxy until the User stops the command.

## Edge cases

- `--socks-port` chooses the port, between 1024 and 65535; otherwise a free port is used.
- SOCKS is the only tunnel type.
