---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#startAction
  - kind: code
    role: implementation
    target: pkg/instance/start.go#Start
  - kind: code
    role: implementation
    target: pkg/hostagent/requirements.go
  - kind: code
    role: implementation
    target: pkg/instance/start.go#StartWithPaths
  - kind: code
    role: implementation
    target: pkg/instance/start.go#watchHostAgentEvents
  - kind: code
    role: implementation
    target: pkg/hostagent/hostagent.go#Run
  - kind: code
    role: implementation
    target: pkg/store/disk.go#LockForInstance
---

# Start an instance

Start a stopped instance: boot the virtual machine, attach its Lima disks, share the configured host directories, start forwarding ports, run its provisioning and wait until its readiness requirements and probes pass, then show its message. `limactl start` with a template or a name that has no instance yet creates the instance first. An instance registered for automatic startup is started by the host's service manager.
