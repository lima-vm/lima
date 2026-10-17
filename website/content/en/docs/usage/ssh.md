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

## The SSH ControlMaster

On hosts other than Windows, the host agent runs the instance's SSH ControlMaster.
Clients that use `ssh.config` share its connection, but never start a master of their own.
When the master exits, the host agent starts a new one and restores the port forwards that Lima manages.
But a forward that Lima did not create, such as the `-D` forward of `limactl tunnel` or one you added with `ssh -O forward`, is lost when the master restarts.

## Using SSH without additional options

Add the following line to your `~/.ssh/config`:

```
Include ~/.lima/*/ssh.config
```

Then you can connect directly without specifying `-F`:
```bash
ssh lima-default
```

This configuration is notably useful for the Remote Development mode of [Visual Studio Code]({{< ref "/docs/examples/vscode" >}}).

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
