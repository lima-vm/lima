# Lima — Pure-Go `cygpath` Replacement (PoC)

A proof-of-concept for the LFX 2026 Term 2 project
[*Improve Windows support (host and guest)*](https://mentorship.lfx.linuxfoundation.org/project/f8bb0ffd-0c84-4cb9-8b66-ea63be76b6e2),
upstream [lima-vm/lima#4907](https://github.com/lima-vm/lima/issues/4907).

This PoC implements the smallest, most-concrete primary deliverable from the
plan: **eliminate the `cygpath.exe` dependency** at
[`pkg/ioutilx/ioutilx.go:54`](../pkg/ioutilx/ioutilx.go#L54) by replacing the
subprocess shell-out with deterministic pure-Go path translation.

## Why this slice

Lima invokes `cygpath` in exactly one place to translate Windows paths into
the form a Git-Bash / MSYS2 / Cygwin SSH client will accept. The shell-out:

- pulls in an implicit Cygwin/MSYS2 dependency on every Windows host,
- silently fails when `cygpath.exe` is missing from `PATH`,
- adds a per-call subprocess on a hot code path.

It is also the single highest-signal change a maintainer can review for this
project — it proves the contributor (a) located the actual call site, (b)
understands what `cygpath -u` does for each path-style namespace, and (c)
can land a drop-in replacement that the existing call site can adopt with a
one-line edit.

## What the PoC does

`pkg/winpath` exposes a drop-in replacement for `ioutilx.WindowsSubsystemPath`:

```go
out, err := winpath.WindowsSubsystemPath(winpath.EnvFromOS(exec.LookPath),
                                         `C:\Users\me\.lima`)
```

Three styles, picked by environment:

| Detected style | Trigger                                        | Output for `C:\Users\me\.lima` |
| -------------- | ---------------------------------------------- | -------------------------------- |
| `native`       | Win32-OpenSSH at `C:\Windows\System32\OpenSSH` | `C:/Users/me/.lima`              |
| `msys`         | `MSYSTEM` env set, or ssh under Git-for-Windows | `/c/Users/me/.lima`              |
| `cygwin`       | `CYGWIN` env set, or ssh under `cygwin64`       | `/cygdrive/c/Users/me/.lima`     |

UNC paths (`\\server\share\...`) pass through with slashes normalized,
matching `cygpath -u`'s behavior for UNC inputs.

Detection precedence (deliberate):

1. `MSYSTEM` env var (user-asserted intent)
2. `CYGWIN` env var
3. `SSH` env var pointing at the ssh binary
4. `exec.LookPath("ssh")` fallback
5. Default: `native`

Path parsing uses pure string logic, not `path/filepath`, so the package is
runnable and testable on any host — the production code path runs only on
Windows, but every branch is exercised in CI on Linux/macOS.

## Layout

```
poc/
├── go.mod
├── README.md
├── cmd/winpath-demo/main.go        # tiny CLI to demo the conversion
└── pkg/winpath/
    ├── winpath.go                  # Style, Env, DetectStyle, Convert, WindowsSubsystemPath
    └── winpath_test.go             # 17 table-driven sub-tests
```

Conventions mirror upstream Lima:

- SPDX header (`Apache-2.0`) on every Go file.
- Package layout under `pkg/` with a sibling `cmd/` binary, matching
  `cmd/lima-driver-*` and `pkg/driver/*` in the parent repo.
- Table-driven tests in `_test.go` next to source, standard `testing` only —
  no extra test deps for this PoC.
- Zero non-stdlib imports.

## Run locally

```bash
cd poc

# build everything
go build ./...

# 17 sub-tests across DetectStyle, Convert, and the end-to-end entry point
go test ./... -v

# demo CLI — three runs to exercise each detected style
go run ./cmd/winpath-demo 'C:\Users\me\.lima'
MSYSTEM=MINGW64 go run ./cmd/winpath-demo 'C:\Users\me\.lima'
CYGWIN=1        go run ./cmd/winpath-demo 'C:\Users\me\.lima'
```

Expected output (last three commands):

```
detected style: native
converted     : C:/Users/me/.lima

detected style: msys
converted     : /c/Users/me/.lima

detected style: cygwin
converted     : /cygdrive/c/Users/me/.lima
```

Requires Go ≥ 1.25 (matches the parent repo's `go.mod`).

## How this would land in `lima-vm/lima`

A real PR would inline `winpath.go` into `pkg/ioutilx/` (the package where
`WindowsSubsystemPath` lives today) and rewrite the call site:

```diff
 // pkg/ioutilx/ioutilx.go
-func WindowsSubsystemPath(ctx context.Context, orig string) (string, error) {
-    out, err := exec.CommandContext(ctx, "cygpath", filepath.ToSlash(orig)).CombinedOutput()
-    if err != nil {
-        logrus.WithError(err).Errorf("failed to convert path to mingw, maybe not using Git ssh?")
-        return "", err
-    }
-    return strings.TrimSpace(string(out)), nil
-}
+func WindowsSubsystemPath(_ context.Context, orig string) (string, error) {
+    return convertWindowsSubsystemPath(envFromOS(), orig)
+}
```

The `WindowsSubsystemPathForLinux` helper on line 62 stays as-is — that one
shells out to `wsl --exec wslpath`, which is correct and not a Cygwin
dependency.

CI gains one new assertion: `strings limactl.exe | grep -c cygpath == 0`.

## Out of scope for this PoC

Everything else in the implementation plan — Windows guest template,
Cloudbase-Init data path, `limactl doctor`, the HCS external driver, WSL2
multi-instance — is intentionally not in this PoC. The point of this slice
is to show one real, runnable, drop-in change end-to-end. The rest of the
plan builds on the same conventions demonstrated here.
