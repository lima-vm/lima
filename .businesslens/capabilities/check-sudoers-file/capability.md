---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers_darwin.go#sudoersAction
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers.go
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers_darwin.go
  - kind: code
    role: implementation
    target: pkg/blockdevice/path.go
---

# Check the sudoers file

Check that an installed sudoers file holds what Lima would generate now. macOS only.
