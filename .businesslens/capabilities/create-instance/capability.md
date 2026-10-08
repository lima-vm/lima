---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#createAction
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#loadOrCreateInstance
  - kind: code
    role: implementation
    target: cmd/limactl/start.go#chooseNextCreatorState
  - kind: code
    role: implementation
    target: pkg/instance/create.go#Create
  - kind: code
    role: implementation
    target: pkg/instance/start.go#Prepare
  - kind: code
    role: implementation
    target: pkg/limatmpl/locator.go#Read
---

# Create an instance

Create a new instance from a template without starting it. The instance is named after the template unless the User names it, and its configuration is the template with its base templates embedded and any settings the User passes on the command line applied on top; Lima's global defaults and overrides apply whenever the configuration is loaded. Lima downloads the images it boots from and prepares its disk.
