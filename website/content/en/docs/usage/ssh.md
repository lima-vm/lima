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

This configuration is notably useful for the Remote Development mode of [Visual Studio Code]({{< ref "/docs/examples/vscode" >}}).

## Using SSH without a config file

If your SSH client does not support a config file, try specifying an equivalent of the following command:

```bash
ssh -p <PORT> -i ~/.lima/_config/user -o UserKnownHostsFile=~/.lima/default/ssh.known_hosts -o HostKeyAlias=lima-default 127.0.0.1
```

The port number can be inspected as follows:
```bash
limactl list --format '{{ .SSHLocalPort }}' default
```

See also `.lima/default/ssh.config`.

## Host key

Lima pins the SSH host key (ed25519) that the guest presents on the first boot after creating the instance,
in `~/.lima/<instance>/ssh.known_hosts`, and verifies the key on the later boots.
The `ssh.config` file refers to this file with `StrictHostKeyChecking=yes`.

If the host key of the guest was changed intentionally (e.g., by reinstalling the SSH server),
remove `~/.lima/<instance>/ssh.known_hosts` and restart the instance to pin the new key.

For an instance created with Lima prior to v2.3, the key is pinned on the first boot with Lima v2.3 or later.
