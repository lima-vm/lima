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

The integration tests are written in [BATS (Bash Automated Testing System)](https://github.com/bats-core/bats-core).

Run the following commands to run the BATS tests:

```bash
git submodule update --init --recursive
make bats
```

The BATS tests are located located under [`hack/bats/tests`](https://github.com/lima-vm/lima/tree/master/hack/bats/tests).

### Extra tests
There are also extra tests ([`hack/bats/extras`](https://github.com/lima-vm/lima/tree/master/hack/bats/extras)) that are not automatically
invoked from `make bats`.

Run the following command to run the extra BATS tests:

```bash
./hack/bats/lib/bats-core/bin/bats ./hack/bats/extras
```

## Template-specific tests

Tests that are specific to template files are written in bash and partially in Perl.

Use [`hack/test-templates.sh`](https://github.com/lima-vm/lima/blob/master/hack/test-templates.sh)
to execute tests, with a virtual machine template file, e.g.,:

```bash
./hack/test-templates.sh ./templates/default.yaml
./hack/test-templates.sh ./templates/fedora.yaml
./hack/test-templates.sh ./hack/test-templates/test-misc.yaml
```

## CI

[`.github/workflows/test.yml`](https://github.com/lima-vm/lima/blob/master/.github/workflows/test.yml)
executes the tests on the GitHub Actions with the ["Tier 1"](../../templates/) templates.

Most tests are executed on Linux runners, as macOS runners are slow and flaky.

The tests about macOS-specific features (e.g., vz and vmnet) are still executed on macOS runners.

Currently, the Intel version of macOS is used, as the ARM version of macOS on GitHub Actions still
do not support nested virtualization.
