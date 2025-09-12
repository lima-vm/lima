---
title: CLI plugins (Experimental)
weight: 2
---

 | ⚡ Requirement | Lima >= 2.0 |
 |----------------|-------------|

Lima supports a plugin-like command aliasing system similar to `git`, `kubectl`, and `docker`. When you run a `limactl` command that doesn't exist, Lima will automatically look for an external program named `limactl-<command>` in your system's PATH and additional directories.

## Plugin Discovery

Lima discovers plugins by scanning for executables named `limactl-<plugin-name>` in the following locations:

1. **Directory containing the `limactl` binary** (including symlink support)
2. **All directories in your `$PATH` environment variable**
3. **`<PREFIX>/libexec/lima`** - For plugins installed by package managers or distribution packages

Plugin discovery respects symlinks, ensuring that even if `limactl` is installed via Homebrew and points to a symlink, all plugins are correctly discovered.

## Plugin Information

Available plugins are automatically displayed in:

- **`limactl --help`** - Shows all discovered plugins with descriptions in an "Available Plugins (Experimental)" section
```bash
Available Plugins (Experimental):
  ps                  Sample limactl-ps alias that shows running instances
  sh
```


- **`limactl info`** - Includes plugin information in the JSON output
```json
{
   "plugins": [
      {
         "name": "ps",
         "path": "/opt/homebrew/bin/limactl-ps"
      },
      {
         "name": "sh",
         "path": "/opt/homebrew/bin/limactl-sh"
      }
   ]
}

```

### Plugin Descriptions

Lima extracts plugin descriptions from script comments using the `<limactl-desc>` format. Include a description comment in your plugin script:

```bash
#!/bin/sh
# <limactl-desc>Docker wrapper that connects to Docker daemon running in Lima instance</limactl-desc>
set -eu

# Rest of your script...
```

**Format Requirements:**
- Only files beginning with a shebang (`#!`) are treated as scripts, and their `<limactl-desc>` lines will be extracted as the plugin description i.e Must contain exactly `<limactl-desc>Description text</limactl-desc>`
- The description text should be concise and descriptive

**Limitations:**
- Binary executables cannot have descriptions extracted and will appear in the help output without a description
- If no `<limactl-desc>` comment is found in a script, the plugin will appear in the help output without a description

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
## How It Works

1. When you run `limactl <unknown-command>`, Lima first tries to find a built-in command
3. If found, Lima executes the external program and passes all remaining arguments to it
4. If not found, Lima shows the standard "unknown command" error

This system allows you to:
- Create personal shortcuts and aliases
- Extend Lima's functionality without modifying the core application
- Share custom commands with your team by distributing scripts
- Package plugins with Lima distributions in the `libexec/lima` directory

## Package Installation

Distribution packages and package managers can install plugins in `<PREFIX>/libexec/lima/` where `<PREFIX>` is typically `/usr/local` or `/opt/homebrew`. This allows plugins to be:
- Managed by the package manager
- Isolated from user's `$PATH`
- Automatically discovered by Lima

## Experimental Status

**Experimental Feature**: The CLI plugin system is currently experimental and may change in future versions. Breaking changes to the plugin API or discovery mechanism may occur without notice.
