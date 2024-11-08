---
title: Testing
weight: 20
---

## Unit tests

The unit tests are written in Go and can be executed with the following commands:

```bash
go test -v ./...
```

The unit tests do not execute actual virtual machines.

## Integration tests

The integration tests incurs actual execution of virtual machines.

The integration tests are written in bash and partially in Perl.

Use [`hack/test-templates.sh`](https://github.com/lima-vm/lima/blob/master/hack/test-templates.sh)
to execute integration tests, with a virtual machine template file, e.g.,:

```bash
./hack/test-templates.sh ./templates/default.yaml
./hack/test-templates.sh ./templates/fedora.yaml
./hack/test-templates.sh ./hack/test-templates/test-misc.yaml
```

## CI

[`.github/workflows/test.yml`](https://github.com/lima-vm/lima/blob/master/.github/workflows/test.yml)
executes the unit tests and the integration tests on the GitHub Actions with the ["Tier 1"](../../templates/) templates.

Most integration tests are executed on Linux runners, as macOS runners are slow and flaky.

The tests about macOS-specific features (e.g., vz and vmnet) are still executed on macOS runners.

Currently, the Intel version of macOS is used, as the ARM version of macOS on GitHub Actions still
do not support nested virtualization.
