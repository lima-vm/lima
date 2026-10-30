---
title: Unmanaged networks (socket_vmnet)
weight: 35
---

| ⚡ Requirement    | macOS |
|-------------------|-------|

Lima can also connect to "unmanaged" networks addressed by "socket". This
means that the daemons will not be controlled by Lima, but must be started
before the instance.  The interface type (host, shared, or bridged) is
configured in `socket_vmnet` and not in lima.

See [Managed networks (macOS)]({{< ref "/docs/config/network/managed/macos#socket_vmnet" >}})
for how to install `socket_vmnet`.

```yaml
networks:
  - socket: "/var/run/socket_vmnet"
```
