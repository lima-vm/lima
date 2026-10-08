---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/tunnel.go#tunnelAction
---

# Create a tunnel

Open a SOCKS proxy on the host through which host programs reach the instance and its network (experimental). The instance is then reachable from a web browser as `lima-NAME.internal`.
