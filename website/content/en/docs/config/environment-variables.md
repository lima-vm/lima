---
title: Environment Variables
weight: 80
---

## Environment Variables

This page documents the environment variables used in Lima.

### `LIMA_INSTANCE`

- **Description**: Specifies the name of the Lima instance to use.
- **Default**: `default`
- **Usage**: 
  ```sh
  export LIMA_INSTANCE=my-instance
  lima uname -a
  ```

### `LIMA_SHELL`

- **Description**: Specifies the shell interpreter to use inside the Lima instance.
- **Default**: User's shell configured inside the instance
- **Usage**: 
  ```sh
  export LIMA_SHELL=/bin/bash
  lima
  ```

### `LIMA_TEMPLATES_PATH`

- **Description**: Specifies the directories used to resolve `template://` URLs.
- **Default**: `$LIMA_HOME/_templates:/usr/local/share/lima/templates`
- **Usage**:
  ```sh
  export LIMA_TEMPLATES_PATH="$HOME/.config/lima/templates:/usr/local/share/lima/templates"
  limactl create --name my-vm template://my-distro
  ```

### `LIMA_WORKDIR`

- **Description**: Specifies the initial working directory inside the Lima instance.
- **Default**: Current directory from the host
- **Usage**: 
  ```sh
  export LIMA_WORKDIR=/home/user/project
  lima
  ```

### `LIMACTL`

- **Description**: Specifies the path to the `limactl` binary.
- **Default**: `limactl` in `$PATH`
- **Usage**: 
  ```sh
  export LIMACTL=/usr/local/bin/limactl
  lima
  ```

### `LIMA_SSH_PORT_FORWARDER`

- **Description**: Specifies to use the SSH port forwarder (slow) instead of gRPC (fast, previously unstable)
- **Default**: `false` (since v1.1.0-beta.0)
- **Usage**: 
  ```sh
  export LIMA_SSH_PORT_FORWARDER=false
  ```
- **Note**: Deprecated since v1.1. It is expected that this variable will be removed in future.
- **The history of the default value**:
  | Version       | Default value       |
  |---------------|---------------------|
  | v0.1.0        | `true`, effectively |
  | v1.0.0        | `false`             |
  | v1.0.1        | `true`              |
  | v1.1.0-beta.0 | `false`             |

### `LIMA_USERNET_RESOLVE_IP_ADDRESS_TIMEOUT`

- **Description**: Specifies the timeout duration for resolving the IP address in usernet.
- **Default**: 2 minutes
- **Usage**: 
  ```sh
  export LIMA_USERNET_RESOLVE_IP_ADDRESS_TIMEOUT=5
  ```

### `_LIMA_QEMU_UEFI_IN_BIOS`

- **Description**: Commands QEMU to load x86_64 UEFI images using `-bios` instead of `pflash` drives.
- **Default**: `false` on Unix like hosts and `true` on Windows hosts
- **Usage**: 
  ```sh
  export _LIMA_QEMU_UEFI_IN_BIOS=true
  ```
- **Note**: It is expected that this variable will be set to `false` by default in future
  when QEMU supports `pflash` UEFI for accelerated guests on Windows.

### `_LIMA_WINDOWS_EXTRA_PATH`

- **Description**: Additional directories which will be added to PATH by `limactl.exe` process to search for tools.
  It is useful, when there is a need to prevent collisions between binaries available in active shell and ones
  used by `limactl.exe` - injecting them only for the running process w/o altering PATH observed by user shell.
  Is is Windows specific and does nothing for other platforms.
- **Default**: unset
- **Usage**:
  ```bat
  set _LIMA_WINDOWS_EXTRA_PATH=C:\Program Files\Git\usr\bin
  ```
- **Note**: It is an experimental setting and has no guarantees being ever promoted to stable. It may be removed
  or changed at any stage of project development.
