---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/main.go#addPluginCommands
  - kind: code
    role: implementation
    target: pkg/plugins/plugins.go#Run
  - kind: code
    role: implementation
    target: pkg/plugins/plugins.go#Find
  - kind: doc
    role: context
    target: website/content/en/docs/config/plugin/cli.md
---

# Run a plugin

Run a command that a `limactl-NAME` executable on the host provides as `limactl NAME` (experimental). Lima finds plugins beside limactl, on `PATH` and in its libexec directory, lists them in its help, and passes the plugin's own arguments, streams and environment through untouched.
