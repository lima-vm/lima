---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/edit.go#editAction
  - kind: code
    role: implementation
    target: cmd/limactl/editflags/editflags.go
  - kind: code
    role: implementation
    target: pkg/limayaml/validate.go#ValidateAgainstLatestConfig
---

# Edit an instance

Change the configuration of an instance that is not running, in the User's editor or with flags and `--set` expressions — its images, resources, mounts, port forwards, networks, Lima disks, host block devices, provisioning and the rest. The changed configuration is validated before it is saved and takes effect the next time the instance starts.
