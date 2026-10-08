---
appliesTo:
  - type: entity
    id: instance
    facts: [Host block devices]
references:
  - kind: code
    role: implementation
    target: pkg/blockdevice/path.go#ValidateDiskDevicePath
  - kind: code
    role: implementation
    target: pkg/limayaml/validate.go
  - kind: doc
    role: context
    target: website/content/en/docs/config/disk.md
---

# Host block devices are distinct macOS disk devices

Each host block device is a normalized absolute macOS disk device path, and no
two of an instance's devices overlap, such as a disk and one of its partitions.
They are attached only to VZ instances on macOS, through a privileged helper the
User authorizes in the sudoers file.
