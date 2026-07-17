---
title: Virtual Machine Drivers
weight: 15
---

 | ⚡ Requirement | Lima >= 2.0 |
 |----------------|-------------|

Lima supports two types of drivers: **internal** and **external**. This architecture allows for extensibility and platform-specific implementations. Drivers are unware whether they are internal or external.

> **💡 See also**: [VM Types](../../config/vmtype) for user configuration of different virtualization backends.


## Internal vs External Drivers

**Internal Drivers** are compiled directly into the `limactl` binary and are registered automatically at startup by passing the driver object into `registry.Register()` function and importing the package in the main limactl code using Go's blank import `_`. For example:
- **qemu:** [registration file](https://github.com/lima-vm/lima/blob/master/pkg/driver/qemu/register.go) & [import file](https://github.com/lima-vm/lima/blob/master/cmd/limactl/main_qemu.go)

Build tags control which drivers are compiled as internal vs external (e.g., `external_qemu`, `external_vz`, `external_wsl2`).

**External Drivers** are separate executables that communicate with Lima via gRPC. They are discovered at runtime from configured directories.

> **⚠️ Note**: External drivers are experimental and the API may change in future releases.

## Building Drivers as External

You can build existing internal drivers as external drivers using the `ADDITIONAL_DRIVERS` Makefile variable:

```bash
# Build QEMU as external driver
make ADDITIONAL_DRIVERS=qemu limactl additional-drivers

# Build multiple drivers as external
make ADDITIONAL_DRIVERS="qemu vz wsl2" limactl additional-drivers
```

This creates external driver binaries in `_output/libexec/lima/` with the naming pattern `lima-driver-<name>` (or `lima-driver-<name>.exe` on Windows).

## Driver Discovery

Lima discovers external drivers from these locations:

1. **Custom directories**: Set path to the external driver's directory via `LIMA_DRIVERS_PATH` environment variable
2. **Standard directory**: `<LIMA-PREFIX>/libexec/lima/`, where `<LIMA_PREFIX>` is the location path where the Lima binary is present

The discovery process is handled by [`pkg/registry/registry.go`.](https://github.com/lima-vm/lima/blob/master/pkg/registry/registry.go)

## Creating Custom External Drivers

To create a new external driver:

1. **Implement the interface**: Your driver must implement the [`driver.Driver`](https://pkg.go.dev/github.com/lima-vm/lima/v2/pkg/driver#Driver) interface:

```go
type Driver interface {
	Lifecycle
	GUI
	SnapshotManager
	GuestAgent

	Info(ctx context.Context) Info
	Configure(ctx context.Context, inst *limatype.Instance) (*ConfiguredDriver, error)
	SSHAddress(ctx context.Context) (string, error)
	AdditionalSetupForSSH(ctx context.Context) error
}
```

   The gRPC transport that carries these methods to an external driver is defined
   in [`driver.proto`](https://github.com/lima-vm/lima/blob/master/pkg/driver/external/driver.proto),
   whose per-RPC doc comments are the authoritative contract (call ordering,
   idempotency and per-RPC semantics) for driver authors. The Go side is
   documented as godoc on the [`driver.Driver`](https://pkg.go.dev/github.com/lima-vm/lima/v2/pkg/driver#Driver)
   interface.

2. **Create main.go**: Use [`server.Serve()`](https://pkg.go.dev/github.com/lima-vm/lima/v2/pkg/driver/external/server#Serve) to expose your driver:

```go
package main

import (
    "context"
    "github.com/lima-vm/lima/v2/pkg/driver/external/server"
)

func main() {
    driver := &MyDriver{}
    server.Serve(context.Background(), driver)
}
```

3. **Build and deploy**:
   - Build your driver: `go build -o lima-driver-mydriver main.go`
   - Place the binary in a directory accessible via `LIMA_DRIVERS_PATH`
   - Ensure the binary is executable

4. **Use the driver**: Explicitly specify the driver when creating instances:

```bash
limactl create myinstance --vm-type=mydriver template:default
```

## Driver lifecycle & call ordering

Drivers are unaware whether they are internal or external, but external driver
authors need to know when each method is invoked. The sequence below is the
instance *start* path: `instance.Prepare` (in `pkg/instance`) runs steps 1-4,
then the hostagent (`pkg/hostagent`) boots the VM and runs steps 5-7:

1. **`Configure`** - sets the instance configuration on the driver. It must run
   before any method that depends on the config.
2. **`Validate`** - rejects a config the driver cannot support. Returning an
   error aborts the start.
3. **`Create`** - first-time provisioning (e.g. writing a driver identifier
   file). It **must** be a no-op (succeed) when the instance already exists, and
   it must **not** create the disks.
4. **`CreateDisk`** - creates the instance disk(s).
5. **`Start`** - begins booting the VM. It is streaming and returns as soon as
   boot has been *initiated* (the initial "started" message), **not** once the
   guest has finished booting; it then streams any runtime errors, then a final
   "done" (see the `StartResponse` message in `driver.proto`).
6. **`AdditionalSetupForSSH`** - runs right after `Start` returns (boot
   initiated, the guest is not necessarily up yet), before the first SSH
   connection.
7. **`SSHAddress`** - re-queried after `Start` only when `Info` reports
   `Features.dynamicSSHAddress` (e.g. WSL2, whose address is not known until the
   VM is up).

The guest agent is reached in one of two ways, chosen by what
**`ForwardGuestAgent`** returns. When it returns `true`, the host agent forwards
the guest-agent socket over SSH and **`GuestAgentConn`** returns `nil`. When it
returns `false`, the driver provides a direct guest-agent connection (e.g.
vsock/virtio) through **`GuestAgentConn`**.

These start-path methods are invoked sequentially in the order above. Creating an
instance without starting it (`limactl create`) only runs `Configure` and
`Create`. Once the VM is running, the host agent polls read-only methods (notably
**`Info`** and **`ForwardGuestAgent`**) from background goroutines, so a driver's
gRPC server must be safe to handle concurrent calls.

For the full per-method contract, see the doc comments in
[`driver.proto`](https://github.com/lima-vm/lima/blob/master/pkg/driver/external/driver.proto)
and the [`driver.Driver`](https://pkg.go.dev/github.com/lima-vm/lima/v2/pkg/driver#Driver) godoc.

## Examples

See existing external driver implementations:
- [`cmd/lima-driver-qemu/main.go`](https://github.com/lima-vm/lima/blob/master/cmd/lima-driver-qemu/main.go)
- [`cmd/lima-driver-vz/main_darwin.go`](https://github.com/lima-vm/lima/blob/master/cmd/lima-driver-vz/main_darwin.go)
- [`cmd/lima-driver-wsl2/main_windows.go`](https://github.com/lima-vm/lima/blob/master/cmd/lima-driver-wsl2/main_windows.go)
