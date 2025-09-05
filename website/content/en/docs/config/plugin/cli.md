---
title: CLI plugins
weight: 2
---

 | ⚡ Requirement | Lima >= 2.0 |
 |----------------|-------------|

Lima supports a plugin-like command aliasing system similar to `git`, `kubectl`, and `docker`. When you run a `limactl` command that doesn't exist, Lima will automatically look for an external program named `limactl-<command>` in your system's PATH.

## Creating Custom Aliases

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

## How It Works

1. When you run `limactl <unknown-command>`, Lima first tries to find a built-in command
2. If no built-in command is found, Lima searches for `limactl-<unknown-command>` in your PATH
3. If found, Lima executes the external program and passes all remaining arguments to it
4. If not found, Lima shows the standard "unknown command" error

This system allows you to:
- Create personal shortcuts and aliases
- Extend Lima's functionality without modifying the core application
- Share custom commands with your team by distributing scripts