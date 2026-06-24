# Hot-Mount for Lima (QEMU/Linux) — Design Spec

- **Date:** 2026-06-24
- **Status:** Approved (brainstorming), pending implementation plan
- **Scope:** Add runtime mount/unmount of host folders into a running Lima VM, with virtiofs as the high-throughput transport.
- **Target:** QEMU driver on Linux hosts. Other drivers report "unsupported" for device hot-plug.

## 1. Goal & motivation

Today a host folder can only be shared with a Lima VM at create/start time. Changing the
mount set requires editing `lima.yaml` and restarting the instance. Users who need to mount a
folder into a *running* VM fall back to ad-hoc transports (e.g. NFS), which suffer from poor
throughput and unbounded guest memory growth.

This feature adds `limactl mount add | remove | list`, letting a user mount and unmount host
folders into a running VM with **no restart**. The primary transport is **virtiofs**, which is
an in-kernel shared filesystem over `vhost-user` + shared memory — dramatically faster than NFS
or sshfs, with a far better memory profile (file data lives in the host page cache instead of
being re-cached in the guest).

### Transport comparison (why virtiofs)

| Transport       | Throughput   | Many-small-files | Guest memory | Role in this feature      |
|-----------------|--------------|------------------|--------------|---------------------------|
| **virtiofs**    | Excellent    | Good             | Best         | **Primary (high-IO)**     |
| 9p              | Medium       | Poor             | OK           | Secondary (same QMP path) |
| reverse-sshfs   | Poor–Medium  | Poor             | OK           | Universal fallback        |
| NFS (status quo)| Poor–Medium  | Poor             | Bad          | The problem being replaced|

DAX (the fastest virtiofs mode) is **out of scope** for v1: default cached virtiofs is already a
large win over NFS, and DAX has historically been less stable and is not always enabled in distro
kernels. DAX is a possible later tuning knob.

## 2. Decisions (locked)

| Decision            | Choice                                                                       |
|---------------------|------------------------------------------------------------------------------|
| Approach            | Hostagent mount-manager + driver-owned QMP hot-plug + SSH guest mount        |
| Transports          | virtiofs (primary), 9p (secondary), reverse-sshfs (fallback)                 |
| Persistence         | **Ephemeral** — hot-mounts are never written to `lima.yaml`; gone on restart |
| Memory backing      | **Option Y** — every QEMU instance boots with shareable memory               |
| virtiofs mode       | Default cached (no DAX)                                                       |
| Driver/host scope   | QEMU on Linux first                                                           |
| Quality bar         | Upstream-quality: unit + integration tests, website docs, changelog          |

### 2a. Implementation refinements (post-brainstorm, discovered during planning)

These refine §5 and §10 after reading the exact code. They reduce risk and surface without
changing the user-facing behavior.

1. **Optional capability interface, not a core `Driver` method.** Hot-plug is exposed via an
   optional `driver.FSHotPlugger` interface (the codebase already uses this pattern for
   `VsockEventEmitter`). Only the QEMU driver implements it. `*driver.ConfiguredDriver` (which
   wraps every driver) gets delegating methods that return `driver.ErrFSHotPlugUnsupported` when
   the wrapped driver lacks the capability. **Consequence:** the VZ, WSL2, and krunkit driver
   files are **not modified** — "unsupported" is automatic.
2. **Linux-gated `memory-backend-memfd`.** On Linux hosts, guest memory is always backed by
   `memory-backend-memfd,share=on` (avoids the `/dev/shm` sizing pitfall of risk 1 and enables
   virtiofs hot-plug on any instance). On non-Linux qemu hosts, the existing `/dev/shm`
   file-backing for static virtiofs mounts is left untouched. PCIe root ports are added only on
   Linux for q35/virt.
3. **External-driver gRPC hot-plug is deferred** (YAGNI). QEMU runs in-process by default and no
   external driver implements hot-plug, so the external `driver.proto` is left unchanged for now.
   The hostagent returns `ErrFSHotPlugUnsupported` for external/non-capable drivers. This is a
   documented follow-up, not a v1 requirement.

## 3. Architecture

```
limactl mount add/remove/list            cmd/limactl/mount.go              (new)
        │  HTTP over ha.sock (unix socket)
        ▼
hostagent HTTP API  POST/DELETE/GET /v1/mounts                            (extend)
        pkg/hostagent/api/{api.go, server/server.go, client/client.go}
        │
        ▼
hostagent mount manager (live registry)  pkg/hostagent/mount.go          (extend)
   ├── reverse-sshfs → existing setupMount()/close()      (no VM change)
   └── 9p / virtiofs:
         1. driver.HotPlugFS(req)   ── QMP device hot-plug   pkg/driver/*  (new method)
         2. ssh.ExecuteScript("mount …")  ── guest-side mount (existing helper)
```

### Layering principles

- **All PCIe topology, QMP commands, and `virtiofsd` lifecycle live in the QEMU driver.** The
  hostagent never speaks QMP directly; it calls `driver.HotPlugFS` / `driver.HotUnplugFS`.
- **The hostagent orchestrates** the two-step add (attach device, then mount in guest) and the
  two-step remove (unmount in guest, then detach device), and tracks the logical mount.
- **The guest agent and its gRPC proto are untouched.** Guest-side mount/unmount uses the
  existing `ssh.ExecuteScript()` helper (with `sudo`), avoiding proto changes and host/guest
  version skew.
- **Other drivers** (VZ, krunkit, WSL2) implement `HotPlugFS` as a stub returning a clear
  "unsupported" error. reverse-sshfs hot-mount is host-side and may be allowed on any driver,
  but only QEMU/Linux is validated in v1.

## 4. Boot-time changes (`pkg/driver/qemu/qemu.go`)

Two changes let *any* running QEMU VM accept a hot-plugged virtio-fs device.

### 4.1 Unconditional shareable memory (Option Y)

Currently the shareable memory backend is added only for virtiofs instances
(`qemu.go:530-533`):

```go
if *y.MountType == limatype.VIRTIOFS {
    args = appendArgsIfNoConflict(args, "-object",
        fmt.Sprintf("memory-backend-file,id=virtiofs-shm,size=%s,mem-path=/dev/shm,share=on", ...))
    args = appendArgsIfNoConflict(args, "-numa", "node,memdev=virtiofs-shm")
}
```

Make the shareable backing **unconditional** so virtiofs can be hot-plugged onto any instance.

> **Risk & mitigation (see §10, risk 1).** `memory-backend-file,mem-path=/dev/shm` ties guest RAM to
> the `/dev/shm` tmpfs size and can fail to boot when guest RAM exceeds it. The recommended fix is
> to switch the backend to **`memory-backend-memfd,share=on`**, which is not bounded by the
> `/dev/shm` mount size. This change must be validated against existing virtiofs instances
> (no regression) and non-virtiofs instances (still boot).

### 4.2 Spare PCIe hot-plug slots

Add **8** `pcie-root-port` controllers at boot:

- x86_64: machine `q35` (`qemu.go:554-569`)
- aarch64: machine `virt` (`qemu.go:571-572`)

Each hot-plugged virtio device binds to a free root port. 8 is the max number of concurrent
hot-mounts; exhausting them yields a clear error. The QEMU driver tracks which ports are free.
Port IDs follow a stable scheme (e.g. `lima-hp-0` … `lima-hp-7`).

## 5. Driver interface change

New methods on the `Driver` interface (`pkg/driver/driver.go`):

```go
// HotPlugFS attaches a 9p or virtiofs device to the running VM and returns an opaque DeviceID.
HotPlugFS(ctx context.Context, req HotPlugFSRequest) (HotPlugFSResponse, error)
// HotUnplugFS detaches a previously hot-plugged device.
HotUnplugFS(ctx context.Context, req HotUnplugFSRequest) error
```

```go
type HotPlugFSRequest struct {
    Type     limatype.MountType // NINEP | VIRTIOFS
    HostPath string
    Tag      string             // MountTag(location, mountPoint)
    Writable bool
    NineP    *limatype.NineP    // protocol version, msize, cache, security model (9p only)
}
type HotPlugFSResponse struct {
    DeviceID string // opaque; used by HotUnplugFS
}
type HotUnplugFSRequest struct {
    DeviceID string
}
```

### Files to touch (internal + external driver)

1. `pkg/driver/driver.go` — interface methods + request/response types.
2. `pkg/driver/external/driver.proto` — RPCs + messages; regenerate `driver.pb.go`, `driver_grpc.pb.go`.
3. `pkg/driver/external/client/methods.go` — client proxy.
4. `pkg/driver/external/server/methods.go` — server dispatch.
5. `pkg/driver/qemu/qemu_driver.go` (+ new `qemu_hotplug.go`) — real implementation.
6. `pkg/driver/vz/vz_driver_darwin.go` — "unsupported" stub.
7. `pkg/driver/wsl2/wsl_driver_windows.go` — "unsupported" stub.
8. `pkg/driver/krunkit/krunkit_driver_darwin_arm64.go` — "unsupported" stub.

### QEMU driver responsibilities

The QEMU implementation **owns**:

- **PCIe-slot allocation** — pick a free `pcie-root-port`, mark it busy; free it on unplug.
- **`virtiofsd` lifecycle** — spawn one `virtiofsd --socket-path <sock> --shared-dir <hostpath>`
  per virtiofs hot-mount (cached mode), track its PID, kill it on unplug.
- **QMP** — reuse the existing on-demand `digitalocean/go-qemu` client pattern
  (`qemu_driver.go:471-483`). It supports raw QMP plus `human-monitor-command` (HMP passthrough).
- A `DeviceID → {rootPort, chardev/fsdev, virtiofsdPID, mountTag}` table for clean teardown.

### QMP sequences

**virtiofs add:**
1. allocate free root port `lima-hp-N`
2. spawn `virtiofsd --socket-path <sock> --shared-dir <hostpath>` (read-only flag if not writable, subject to virtiofsd RO support)
3. `chardev-add` socket backend (`server=false`; QEMU is the client, virtiofsd the server)
4. `device_add vhost-user-fs-pci,id=<dev>,chardev=<char>,tag=<tag>,bus=lima-hp-N,queue-size=1024`

**9p add:**
1. allocate free root port `lima-hp-N`
2. `human-monitor-command: fsdev_add local,id=<fsdev>,path=<hostpath>,security_model=none[,readonly=on]`
3. `device_add virtio-9p-pci,id=<dev>,fsdev=<fsdev>,mount_tag=<tag>,bus=lima-hp-N`

**remove (both):**
1. (caller already unmounted in guest)
2. `device_del <dev>`
3. **wait for `DEVICE_DELETED`** event — PCIe unplug is asynchronous and needs guest cooperation; time out if the guest will not release the device
4. `chardev-remove <char>` (virtiofs) or `human-monitor-command: fsdev_del <fsdev>` (9p)
5. kill `virtiofsd` (virtiofs); free the root port

## 6. Hostagent mount manager (`pkg/hostagent/mount.go`)

Today mounts live only in a teardown stack (`hostagent.go:583-597`). Add a concurrent-safe
registry keyed by guest mountpoint:

```go
type activeMount struct {
    id         string
    mountPoint string             // guest path; user-facing key
    mountType  limatype.MountType
    close      func() error       // reverse-sshfs teardown
    deviceID   string             // 9p/virtiofs: opaque id from HotPlugFS
}
type mountManager struct {
    mu     sync.Mutex
    mounts map[string]*activeMount // keyed by mountPoint
}
```

**`MountAdd(ctx, req)`**
- reverse-sshfs → `setupMount()`, register.
- 9p/virtiofs → `driver.HotPlugFS(req)`; then `ssh.ExecuteScript` runs in the guest:
  `sudo mkdir -p <mp> && sudo mount -t <fstype> -o <opts> <tag> <mp>`; then register.
- **Rollback:** if the guest mount fails after the device is attached, call `driver.HotUnplugFS`
  before returning the error, so no orphan device is left.

**`MountRemove(ctx, mountPoint)`**
- reverse-sshfs → `close()`.
- 9p/virtiofs → `ssh.ExecuteScript("sudo umount <mp>")`; then `driver.HotUnplugFS(deviceID)`.
- Remove from registry.

**`MountList()`** → snapshot of the registry.

**Shutdown:** the existing cleanup path closes all registry entries (and the QEMU driver kills any
`virtiofsd` and frees ports). Nothing is persisted — ephemeral by design.

### Shared mount-option builder

The 9p/virtiofs mount-option strings are currently built inline in `pkg/cidata/cidata.go:212-240`
for boot-time fstab. Factor this into a single reusable helper (e.g.
`limayaml.MountOptions(m, mountType)` or a small `mountutil` package) so boot-time mounts and
runtime SSH mounts share one source of truth and cannot drift. Concrete strings to reproduce:

- 9p: `<rw|ro>,trans=virtio,version=9p2000.L,msize=<bytes>,cache=<cache>,nofail`
- virtiofs: `<rw|ro>,nofail`

## 7. CLI & API

### CLI (`cmd/limactl/mount.go`, registered in `cmd/limactl/main.go`)

```
limactl mount add    <instance> <host-path> <guest-path> [--type virtiofs|9p|reverse-sshfs] [--writable]
limactl mount remove <instance> <guest-path>
limactl mount list   <instance>
```

- `--type` defaults to `virtiofs` on QEMU/Linux.
- Instance must be running. Resolve via `store.Inspect`; connect via
  `hostagentclient.NewHostAgentClient(ha.sock)` (pattern from `pkg/store/instance.go:72-88`).
- Follow the cobra structure of `shell`/`edit` (parent command + subcommands, bash completion of
  instance names).

### HostAgent HTTP API (`pkg/hostagent/api/`)

Extend the server that today serves only `/v1/info`:

- `POST   /v1/mounts`  body `{hostPath, guestPath, type, writable}` → `{id, mountPoint, type}`
- `DELETE /v1/mounts`  body `{guestPath}` → `204`
- `GET    /v1/mounts`  → `[{id, mountPoint, type, hostPath, writable}]`

Mirror the existing `Info` client/server/route pattern (`api.go`, `server/server.go`,
`client/client.go`).

## 8. Error handling & edge cases

- Instance not running → clear error.
- Non-QEMU driver + `--type 9p|virtiofs` → "device hot-plug unsupported on this driver".
- No free PCIe slot → "max concurrent hot-mounts reached (8)".
- Host path missing / not a directory → reject before attaching.
- Guest mountpoint already mounted, or a system-protected path (`/`, `/etc`, `/usr`, …) → reject,
  reusing `pkg/limayaml/validate.go:129-138` rules.
- Duplicate guest path already in the registry → reject (no silent replace).
- Hot-unplug "device busy" / no `DEVICE_DELETED` within timeout → return error, leave consistent
  state (device stays plugged; registry unchanged).
- `virtiofsd` binary not found (virtiofs only) → clear error.

## 9. Testing

### Unit

- mount manager: add/remove/list, concurrency, slot exhaustion, rollback — against a fake driver.
- QMP command-string construction for 9p and virtiofs add/remove.
- shared mount-option builder (table tests for rw/ro, msize, cache).
- PCIe slot allocator (allocate/free/exhaust).
- CLI argument parsing and validation.
- API handlers via `httptest`.

### Integration (`hack/`, Linux + QEMU gated)

Mirror `hack/test-mount-home.sh` style:
1. start a QEMU/Linux instance,
2. `limactl mount add` a virtiofs folder, write a file on the host, read it in the guest (and
   vice-versa), sanity-check throughput,
3. `limactl mount remove`, verify the mountpoint is gone,
4. repeat for 9p and reverse-sshfs.

## 10. Risks & open questions

1. **Unconditional shareable memory (highest risk).** See §4.1. `memory-backend-file` on
   `/dev/shm` can break boot when guest RAM > `/dev/shm`. Recommended path: switch to
   `memory-backend-memfd,share=on`; validate no regression for existing virtiofs instances and
   that non-virtiofs instances still boot. This must be proven before the feature is considered
   safe to merge.
2. **PCIe hotplug kernel support.** Requires `CONFIG_HOTPLUG_PCI_PCIE` in the guest kernel
   (standard in Ubuntu and the default Lima images) — verify on the default image.
3. **`fsdev_add` / `fsdev_del` are HMP-only** (issued via `human-monitor-command`) — confirm
   availability across the QEMU versions Lima targets.
4. **Hot-unplug guest cooperation** — `DEVICE_DELETED` may never arrive if the mount is busy;
   needs a timeout and a clear error, plus guidance to unmount cleanly first.
5. **`virtiofsd` lifecycle** for hot-added shares (spawn/track/kill), including when QEMU runs as
   the external `lima-driver-qemu` process (child processes owned by the driver process).
6. **Partial-failure rollback** — every multi-step add must unwind cleanly on failure at any step.

## 11. Non-goals (v1)

- Persisting hot-mounts to `lima.yaml` (ephemeral only).
- 9p/virtiofs hot-plug on VZ, krunkit, or WSL2 (VZ has no hot-plug API).
- virtiofs DAX mode.
- macOS or Windows hosts.
- Re-mounting or changing options of an already-active mount (remove + add instead).

## 12. Affected files (summary)

- `pkg/driver/qemu/qemu.go` — unconditional shareable memory; spare `pcie-root-port`s.
- `pkg/driver/qemu/qemu_driver.go` (+ new `qemu_hotplug.go`) — QMP hot-plug, slot + virtiofsd mgmt.
- `pkg/driver/driver.go` — `HotPlugFS`/`HotUnplugFS` + types.
- `pkg/driver/external/{driver.proto, client/methods.go, server/methods.go}` (+ regen pb).
- `pkg/driver/{vz,wsl2,krunkit}/*` — "unsupported" stubs.
- `pkg/hostagent/mount.go` — mount manager registry + add/remove/list.
- `pkg/hostagent/hostagent.go` — wire registry into lifecycle/cleanup.
- `pkg/hostagent/api/{api.go, server/server.go, client/client.go}` — `/v1/mounts` endpoints.
- `pkg/cidata/cidata.go` + new shared helper — factor mount-option builder.
- `cmd/limactl/mount.go`, `cmd/limactl/main.go` — CLI.
- `website/content/en/docs/config/mount.md`, changelog — docs.
- `hack/` + `*_test.go` — integration + unit tests.

---

## Addendum: VZ (macOS) hot-mount — implemented 2026-06-24

VZ's Virtualization.framework has **no device hot-plug API**, so the QEMU approach
(hot-plug a new device per mount) cannot work. Instead VZ exploits the **mutable
`VZVirtioFileSystemDevice.share`** property, validated on macOS 26 / Apple M-series:

- **Boot-time** (`pkg/driver/vz/vm_darwin.go`): reserve **8 spare empty virtio-fs devices**
  (tags `lima-hotmount-0..7`, indices 0..7), each sharing an empty placeholder dir — the VZ
  analog of QEMU's reserved PCIe root ports.
- **Runtime** (`pkg/driver/vz/hotplug_darwin.go`, implementing `driver.FSHotPlugger`):
  `HotPlugFS` allocates a free slot and sets that device's share to the host folder via a new
  binding method `(*vz.VirtualMachine).SetVirtioFileSystemDeviceShareAtIndex`. The guest then
  mounts the device's fixed tag (returned via `HotPlugFSResponse.Tag`) over SSH.
  `HotUnplugFS` clears the share back to the placeholder and frees the slot.
- **Binding extension**: the upstream `Code-Hex/vz` binding only exposes share-setting on the
  *configuration*, so a small runtime method was added (vendored in `third_party/Code-Hex-vz`,
  isolated patch in `hack/patches/code-hex-vz-runtime-share.patch`, to be upstreamed).

**Verified on native VZ (macOS 26, M4):** hot-mount/unmount, two concurrent mounts, slot reuse,
writable vs read-only, and **~5 GB/s** sequential read — identical to a static virtiofs mount.
The hostagent, `/v1/mounts` API, and `limactl mount` command were unchanged: VZ slots in purely
by implementing the existing `FSHotPlugger` capability interface.
