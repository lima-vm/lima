---
title: BATS Style Guide
weight: 25
---

Lima uses [BATS](https://bats-core.readthedocs.io/) with the
[bats-support](https://github.com/bats-core/bats-support),
[bats-assert](https://github.com/bats-core/bats-assert), and
[bats-file](https://github.com/bats-core/bats-file) helper libraries.

All tests run with `errexit` enabled (via `BATS_RUN_ERREXIT=1` in
`helpers/load.bash`), so any failing command aborts the test immediately.

## When to use `run`

Use `run` only when you need to capture output or assert a non-zero exit code.
Do not use it just to check that a command succeeds.

### Command should succeed, output does not matter

Call the command directly. `errexit` handles the failure case.

```bash
# Good
limactl shell "$INSTANCE" -- mkdir -p /tmp/foo

# Bad — unnecessary run and status check
run limactl shell "$INSTANCE" -- mkdir -p /tmp/foo
[[ $status == 0 ]]

# Bad — unnecessary run and assert_success
run limactl shell "$INSTANCE" -- mkdir -p /tmp/foo
assert_success
```

### Command should succeed, and you need its output

Use `run -0` to assert success and capture `$output`/`$lines`, then use
`assert_output` or `assert_line` to verify the output.

```bash
# Good
run -0 limactl shell "$INSTANCE" -- cat /tmp/hello.txt
assert_output "hello"

# Bad — manual status and output checks
run limactl shell "$INSTANCE" -- cat /tmp/hello.txt
[[ $status == 0 ]]
[[ $output == "hello" ]]
```

### Command should fail with a specific exit code

Use `run -N` where `N` is the expected exit code.

```bash
run -1 limactl yq -n foo
assert_output --partial "invalid input"
```

## Checking stderr (log messages)

Use `run_e` (a wrapper for `run --separate-stderr`) when you need to check
both stdout and stderr. The helpers `assert_fatal`, `assert_warning`,
`assert_info`, `assert_error`, and `assert_debug` match Lima's structured
log output in stderr.

```bash
run_e -1 limactl ls foo foobar bar
assert_warning 'No instance matching foobar found.'
assert_fatal 'unmatched instances'
```

## Checking files and directories

Use bats-file assertions instead of test expressions.

```bash
# Good
assert_file_exists "$LIMA_HOME/$INSTANCE/protected"
assert_dir_exists "$BATS_TEST_TMPDIR/foo/bar"

# Bad — raw test expressions give poor failure messages
[[ -f "$LIMA_HOME/$INSTANCE/protected" ]]
[[ -d "$BATS_TEST_TMPDIR/foo/bar" ]]
```

## Test lifecycle

Define `local_setup_file`, `local_teardown_file`, `local_setup`, and
`local_teardown` instead of overriding `setup_file`, `setup`, etc. directly.
The base implementations in `helpers/load.bash` call these `local_` variants
automatically.

Set `INSTANCE` at file scope to have `setup_file` create (or reuse) a Lima
instance and `teardown_file` delete it.

## Summary

| Goal | Pattern |
|---|---|
| Command must succeed, ignore output | `limactl ...` (bare command) |
| Command must succeed, check output | `run -0 cmd; assert_output ...` |
| Command must fail | `run -N cmd; assert_output ...` |
| Command must fail, check stderr | `run_e -N cmd; assert_fatal ...` |
| Command must succeed, check stderr | `run_e -0 cmd; assert_info ...` |
| File or directory exists | `assert_file_exists` / `assert_dir_exists` |
