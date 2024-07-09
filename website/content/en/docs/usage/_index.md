---
title: Usage
weight: 2
---

## Starting a Linux instance

Run `limactl start <INSTANCE>` to create and start the first instance.
The `<INSTANCE>` name defaults to "default".

```console
$ limactl start
? Creating an instance "default"  [Use arrows to move, type to filter]
> Proceed with the current configuration
  Open an editor to review or modify the current configuration
  Choose another template (docker, podman, archlinux, fedora, ...)
  Exit
...
INFO[0029] READY. Run `lima` to open the shell.
```

Choose `Proceed with the current configuration`, and wait until "READY" to be printed on the host terminal.

For automation,  `--tty=false` flag can be used for disabling the interactive user interface.

### Customization
To create an instance "default" from a template "docker":
```bash
limactl create --name=default template://docker
limactl start default
```

See also the command reference:
- [`limactl create`](../reference/limactl_create/)
- [`limactl start`](../reference/limactl_start/)
- [`limactl edit`](../reference/limactl_edit/)

### Executing Linux commands
Run `limactl shell <INSTANCE> <COMMAND>` to launch `<COMMAND>` on the VM:
```bash
limactl shell default uname -a
```

See also the command reference:
- [`limactl shell`](../reference/limactl_shell/)

For the "default" instance, this command can be shortened as `lima <COMMAND>`.
```bash
lima uname -a
```
The `lima` command also accepts the instance name as the environment variable `$LIMA_INSTANCE`.


SSH can be used too:
```console
$ limactl ls --format='{{.SSHConfigFile}}' default
/Users/example/.lima/default/ssh.config

$ ssh -F /Users/example/.lima/default/ssh.config lima-default
```

### Shell completion
- To enable bash completion, add `source <(limactl completion bash)` to `~/.bash_profile`.
- To enable zsh completion, see `limactl completion zsh --help`
