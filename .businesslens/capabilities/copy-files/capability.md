---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/copy.go#copyAction
  - kind: code
    role: implementation
    target: pkg/copytool/copytool.go#New
  - kind: code
    role: implementation
    target: pkg/copytool/copytool.go#parseCopyPaths
---

# Copy files

Copy files and directories between the host and running instances, or between two instances through the host, using rsync when the host and every guest involved have it and scp otherwise.
