---
appliesTo:
  - type: entity
    id: lima-disk
    facts: [Size]
  - type: entity
    id: instance
    facts: [Disk size]
references:
  - kind: code
    role: implementation
    target: pkg/limayaml/validate.go#ValidateAgainstLatestConfig
  - kind: code
    role: implementation
    target: cmd/limactl/disk.go#diskResizeAction
---

# Disks never shrink

Neither an instance's disk size nor a Lima disk can be made smaller than it is.
Growing an instance's disk size takes effect when it next starts.
