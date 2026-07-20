# AGENTS.md

Guidance for AI coding agents working in the Lima repository. The full contribution policy lives in
[`contributing.md`](website/content/en/docs/community/contributing.md); this file highlights what
matters most to agents and points to the authoritative docs instead of copying them, so it stays in
sync with the code.

## AI contribution rules

A summary of the "AI Contribution Rules" section of
[`website/content/en/docs/community/contributing.md`](website/content/en/docs/community/contributing.md#ai-contribution-rules), which
is the source of truth.

- One fix per pull request.
- Humans are responsible for all content - a human reviews and edits any AI-generated code and pull
  request text, and replies to review comments themselves.
- DCO: agents MUST NOT add a `Signed-off-by` trailer. Only the human submitting the code signs off,
  with `git commit -s`.
- Disclose AI usage with an `Assisted-by: AI_TOOL_NAME` trailer in the pull request description.

## Build, test, lint

Lima is `github.com/lima-vm/lima/v2` (imports use the `/v2` suffix) and requires Go 1.25+. The build
uses a GNU Makefile; output goes to `_output/`.

```bash
make native        # build limactl + native guestagent + templates (fastest full dev build)
make minimal       # build just limactl + native guestagent + default template
go test ./...      # unit tests - never boot VMs
make bats          # integration tests (BATS); boot real VMs (needs git submodules)
make lint          # editorconfig, golangci-lint, yamllint, ls-lint, shellcheck, ltag, ...
make generate      # regenerate protobuf after editing a .proto file
```

Unit tests never execute VMs; anything that boots a VM is a BATS or template test under `hack/`
(for example `./hack/test-templates.sh ./templates/default.yaml`). Every commit must be signed off
with `git commit -s`, or CI fails.

## Where things are

Pointers to the authoritative sources - read these rather than a duplicated copy here.

- Architecture, the three processes (`limactl` / hostagent / guestagent), the on-disk `${LIMA_HOME}`
  layout, and every `LIMA_CIDATA_*` variable:
  [`website/content/en/docs/dev/internals.md`](website/content/en/docs/dev/internals.md).
- Config / data model: `pkg/limatype` (core `LimaYAML` / `Instance` types), `pkg/limayaml`
  (load / default / validate), `pkg/limatmpl` and `pkg/templatestore` (templates).
- Drivers (virtualization backends): `pkg/driver/` (`qemu`, `vz`, `krunkit`, `wsl2`, and `external`
  for the gRPC bridge).
- Guest provisioning: `pkg/cidata/` builds `cidata.iso` (guestagent binary, boot scripts, and the
  `user-data` file). `user-data` uses the cloud-config YAML format defined by cloud-init, which a
  guest may consume with an implementation other than Python cloud-init.
- Instance lifecycle: `pkg/instance/`.

## Conventions

- Cross-platform code is split across many `_darwin.go` / `_linux.go` / `_windows.go` / `_others.go`
  / `_unix.go` files; when changing such behavior, update every variant, not just one.
- New Go, shell, Dockerfile, and Makefile files need an SPDX header (`ltag` enforces this in CI;
  other file types, including markdown, are exempt).
- Keep the `gomodjail` / `gosocialcheck` annotations in `go.mod`.
- Reuse the small `pkg/*util` helpers (`fsutil`, `osutil`, `sshutil`, ...) instead of reimplementing.
