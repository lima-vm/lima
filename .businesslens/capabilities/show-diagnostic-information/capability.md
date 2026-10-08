---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/info.go#infoAction
  - kind: code
    role: implementation
    target: pkg/limainfo/limainfo.go#New
---

# Show diagnostic information

Show, as JSON, Lima's version, home directory, available VM types and guest agents, the templates and plugins it finds, the default template's filled-in configuration and the environment variables `--preserve-env` holds back.
