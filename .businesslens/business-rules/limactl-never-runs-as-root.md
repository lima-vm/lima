---
appliesTo:
  - type: context
    context:
      place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/main.go#main
---

# limactl never runs as the root user

Every limactl command refuses to run as the root user; commands that need
privileges ask for sudo themselves or print what the User installs with sudo.
