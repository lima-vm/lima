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

#### Using SSH without the `-F` flag

To connect directly without specifying the config file, add this to your `~/.ssh/config`:

```
Include ~/.lima/*/ssh.config
```

Then you can connect directly:
```bash
ssh lima-default
```

### Command Aliasing (Plugin System)

Lima supports a plugin-like command aliasing system similar to `git`, `kubectl`, and `docker`. When you run a `limactl` command that doesn't exist, Lima will automatically look for an external program named `limactl-<command>` in your system's PATH.

#### Creating Custom Aliases

To create a custom alias, create an executable script with the name `limactl-<alias>` and place it somewhere in your PATH.

**Example: Creating a `ps` alias for listing instances**

1. Create a script called `limactl-ps`:
   ```bash
   #!/bin/sh
   # Show instances in a compact format
   limactl list --format table "$@"
   ```

2. Make it executable and place it in your PATH:
   ```bash
   chmod +x limactl-ps
   sudo mv limactl-ps /usr/local/bin/
   ```

3. Now you can use it:
   ```bash
   limactl ps                    # Shows instances in table format
   limactl ps --quiet            # Shows only instance names
   ```

**Example: Creating an `sh` alias**

```bash
#!/bin/sh
# limactl-sh - Connect to an instance shell
limactl shell "$@"
```

After creating this alias:
```bash
limactl sh default           # Equivalent to: limactl shell default
limactl sh myinstance bash   # Equivalent to: limactl shell myinstance bash
```

#### How It Works

1. When you run `limactl <unknown-command>`, Lima first tries to find a built-in command
2. If no built-in command is found, Lima searches for `limactl-<unknown-command>` in your PATH
3. If found, Lima executes the external program and passes all remaining arguments to it
4. If not found, Lima shows the standard "unknown command" error

This system allows you to:
- Create personal shortcuts and aliases
- Extend Lima's functionality without modifying the core application
- Share custom commands with your team by distributing scripts

## Understanding Lima's Operation Modes

Lima operates in different modes that affect how it integrates with your host system. By default, Lima runs in "integrated mode" which automatically mounts your home directory, forwards ports, and sets up container engines. For users who prefer more control or isolation, "plain mode" is available.

See [Operation Modes](./operation-modes) for a detailed explanation of these modes and when to use each one.

### Shell completion
- To enable bash completion, add `source <(limactl completion bash)` to `~/.bash_profile`.
- To enable zsh completion, see `limactl completion zsh --help`
