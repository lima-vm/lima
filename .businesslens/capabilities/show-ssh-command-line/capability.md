---
availability:
  - place: limactl
references:
  - kind: code
    role: implementation
    target: cmd/limactl/show-ssh.go#showSSHAction
---

# Show the SSH command line

Print the `ssh` command line, arguments or options that reach an instance, for use outside Lima. The command is deprecated in favour of `ssh -F $LIMA_HOME/NAME/ssh.config lima-NAME`.
