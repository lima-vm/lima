---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/screenshot.go#screenshotAction
  - kind: code
    role: implementation
    target: pkg/hostagent/hostagent.go#Screenshot
---

# Capture a screenshot

Save an image of a running instance's display, for instances that have a display and run on QEMU or VZ.
