---
type: cli
actors:
  - user
entryPoints:
  - cli: limactl
  - cli: lima
  - cli: nerdctl.lima
  - cli: docker.lima
  - cli: podman.lima
  - cli: kubectl.lima
  - cli: apptainer.lima
references:
  - kind: code
    role: implementation
    target: cmd/limactl/main.go#newApp
  - kind: code
    role: implementation
    target: cmd/lima
  - kind: doc
    role: context
    target: website/content/en/docs/reference/_index.md
---

# limactl

The command line through which the User creates, runs and works with instances
and their snapshots, disks, networks and templates. `lima` opens a shell in the
instance named by `LIMA_INSTANCE` (`default` unless set), and the `*.lima`
wrappers run nerdctl, Docker, Podman, kubectl or Apptainer against that
instance. Commands ask questions and open an editor only when a terminal is
attached (`--tty`); otherwise, as in scripts, they proceed without asking.
