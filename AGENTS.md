# AGENTS.md

Guidance for AI coding agents working in the Lima repository. It points to the authoritative docs
instead of duplicating them, so it stays in sync with the code.

## AI contribution rules

Read @website/content/en/docs/community/contributing.md and follow its "AI Contribution Rules"
section.

## Build, test, lint

Lima is `github.com/lima-vm/lima/v2` (imports use the `/v2` suffix). The build
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
- Drivers (virtualization backends): `pkg/driver/`
- Guest provisioning: `pkg/cidata/` builds `cidata.iso` (guestagent binary, boot scripts, and the
  `user-data` file). `user-data` uses the cloud-config YAML format defined by cloud-init, which a
  guest may consume with an implementation other than Python cloud-init.
- Instance lifecycle: `pkg/instance/`.

## Conventions

- New Go, shell, Dockerfile, and Makefile files need an SPDX header (`ltag` enforces this in CI;
  other file types, including markdown, are exempt).
- Keep the `gomodjail` / `gosocialcheck` annotations in `go.mod`.
