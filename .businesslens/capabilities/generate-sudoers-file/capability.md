---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers.go
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers_darwin.go#sudoersAction
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers_nodarwin.go
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers_nodarwin.go#sudoersAction
  - kind: doc
    role: context
    target: website/content/en/docs/config/network/vmnet.md
  - kind: code
    role: implementation
    target: cmd/limactl/sudoers_darwin.go
  - kind: code
    role: implementation
    target: pkg/blockdevice/path.go
---

# Generate the sudoers file

Print the `/etc/sudoers.d/lima` content that lets Lima start the privileged helpers for its shared, bridged and host networks and, for the host block devices named, for the current user. Installing it is left to the User. macOS only.
