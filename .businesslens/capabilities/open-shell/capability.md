---
availability:
  - place: limactl
domain: basic-commands
references:
  - kind: code
    role: implementation
    target: cmd/limactl/shell.go#shellAction
  - kind: code
    role: implementation
    target: cmd/lima
  - kind: code
    role: implementation
    target: cmd/nerdctl.lima
  - kind: code
    role: implementation
    target: cmd/limactl/shell.go#askUserForRsyncBack
  - kind: code
    role: implementation
    target: cmd/docker.lima
  - kind: code
    role: implementation
    target: pkg/envutil/envutil.go
  - kind: code
    role: implementation
    target: pkg/mcp/toolset/shell.go#RunShellCommand
---

# Open a shell

Open a login shell in a running instance, or run one command there, starting in the same directory as on the host when that directory is shared with the guest. `lima COMMAND` does this for the instance named by `LIMA_INSTANCE`, and the `*.lima` wrappers run a container or Kubernetes command the same way.
