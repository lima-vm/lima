---
availability:
  - place: limactl
domain: lima-disks
references:
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskCreateAction
  - kind: code
    role: implementation
    target: pkg/imgutil/proxyimgutil/proxyimgutil.go
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go
  - kind: code
    role: implementation
    target: pkg/store/disk.go
---

# Create a Lima disk

Create an empty Lima disk of a given size. `--format` accepts `qcow2` or `raw`, but the disk is made in qemu-img's default format when qemu-img is installed, and raw otherwise.
