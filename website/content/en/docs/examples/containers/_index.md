---
title: Containers
weight: 3
---

Lima was designed to facilitate running containers inside a virtual machine, with automatic
[filesystem sharing]({{< ref "/docs/config/mount" >}}) and  [port forwarding]({{< ref "/docs/config/port" >}}).

The original motivation of Lima was to promote [containerd]({{< ref "/docs/examples/containers/containerd" >}}) for macOS users, however,
the current version of Lima supports other container engines too, and does not depend on macOS hosts.
