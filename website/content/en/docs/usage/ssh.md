---
title: SSH
weight: 3
---

Instead of the `limactl shell` command, SSH can be used too:

```console
$ limactl ls --format='{{.SSHConfigFile}}' default
/Users/example/.lima/default/ssh.config

$ ssh -F /Users/example/.lima/default/ssh.config lima-default
```

This is useful for interoperability with other software that expects the SSH connectivity.

## Using SSH without additional options

Add the following line to your `~/.ssh/config`:

```
Include ~/.lima/*/ssh.config
```

Then you can connect directly without specifying `-F`:
```bash
ssh lima-default
```

This configuration is notably useful for the Remote Development mode of [Visual Studio Code](../examples/vscode.md).

## Using SSH without a config file

If your SSH client does not support a config file, try specifying an equivalent of the following command:

```bash
ssh -p <PORT> -i ~/.lima/_config/user -o NoHostAuthenticationForLocalhost=yes 127.0.0.1
```

The port number can be inspected as follows:
```bash
limactl list --format '{{ .SSHLocalPort }}' default
```

See also `.lima/default/ssh.config`.