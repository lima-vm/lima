# Templates

A template is the YAML an instance is created from, and the unit Lima users share with each
other. The `limactl template` subcommands used below are marked experimental
(<https://lima-vm.io/docs/releases/experimental/>), so their flags and output may change
between releases. Reference: <https://lima-vm.io/docs/templates/> and
<https://lima-vm.io/docs/config/>.

## Decide

| You need | Do this |
|----------|---------|
| A small change to a stock template, once | `--set '<yq expression>'` at creation |
| A value the template already exposes | `--param name=value` |
| A reusable variant of a stock template | a new file with a `base:` locator |
| To edit an existing instance's config | `limactl edit <inst>` |
| To share a template with one other person | send the file, or a `https://` URL |
| To share from a Git repository | a `github:` locator |
| A template that works with no network lookups | `limactl template copy --embed` |

Prefer `--param` over `--set` where the template declares the parameter: it is the supported
surface, and it survives template changes.

## Locators

A locator is anything `limactl start` and `base:` accept.

| Form | Example |
|------|---------|
| Bundled template | `template:fedora` |
| Local file | `./my.yaml`, `file:///abs/path.yaml` |
| URL | `https://example.com/my.yaml` |
| Git repository, experimental | `github:lima-vm/lima/templates/fedora` |

`github:` locators accept `@TAG` for a branch, tag or commit, follow symlinks inside the
repository, and resolve the default branch through the GitHub API when no tag is given. Set
`GITHUB_TOKEN` or `GH_TOKEN` when resolving them repeatedly, or the rate limit will stop you.
Full rules: <https://lima-vm.io/docs/templates/github/>.

In Lima 1.x these were written `template://fedora`. Lima 2 uses `template:fedora`.

## Do

```bash
# the fastest way to see exactly what a locator will give you,
# with base: layers resolved and defaults filled in
limactl template copy --fill template:default -

# query one field without parsing YAML yourself
limactl template yq template:default '.images[].location'

# check a template before using it
limactl template validate ./my.yaml

# see what a locator resolves to
limactl template url github:lima-vm/lima/templates/fedora

# produce a self-contained template with external references inlined
limactl template copy --embed ./my.yaml ./my-standalone.yaml

# use it
limactl start -y --name dev ./my-standalone.yaml
```

A minimal derived template. Note the base: `template:fedora` is itself
`_images/fedora` plus `_default/mounts`, and **mount lists from `base:` are appended, not
replaced**, so basing on it would mount all of `~` as well as `~/src`. Basing on the image
template instead keeps the mount list to what you declare:

```yaml
base:
- template:_images/fedora
cpus: 4
memory: 8GiB
mounts:
- location: "~/src"
  writable: true
```

There is no YAML equivalent of `--mount-none`, so always check what you actually inherited.
`template yq` uses Lima's built-in evaluator, so it needs no external `yq`:

```bash
limactl template yq ./my.yaml '.mounts'
```

## Installing software in the guest

`provision:` is how a template puts software in the guest. There is no page for it on
lima-vm.io; the annotated reference is the commented block in the bundled `default.yaml`,
which `limactl template copy template:default -` prints.

| Mode | Runs | Use for |
|------|------|---------|
| `system` | as root, after boot | installing packages, system config |
| `user` | as the instance user, after `system` | dotfiles, per-user tool installs |
| `boot` | by `/bin/sh` in early cloud-init, before the rest | disabling a unit before it delays boot; takes no hash-bang |
| `dependency` | before Lima's own package resolution | adding a repository the later steps need |
| `data` | not executed; written to the guest filesystem | dropping a config file in place |

```yaml
base:
- template:_images/fedora
provision:
- mode: system
  script: |
    #!/bin/bash
    set -eux -o pipefail
    dnf install -y make gcc
- mode: data
  path: /etc/myapp/config.toml
  owner: root:root
  permissions: "0644"
  content: |
    verbose = true
```

Scripts run once, on first boot. Changing `provision:` does not re-run them on an existing
instance; recreate it. A failure here does not stop the instance from starting, so the VM can
come up with the software missing. Check `/var/log/cloud-init-output.log` in the guest when
something you provisioned is not there.

`mode: ansible` is deprecated. Run `ansible-playbook` yourself instead.

## Distribution

`limactl` searches for named templates in `$LIMA_HOME/_templates` first, then in
`<prefix>/share/lima/templates`, the bundled set.

Setting `$LIMA_TEMPLATES_PATH` **replaces** that list rather than prepending to it, so a value
that omits the bundled directory makes `template:default` unresolvable.

Dropping a file into `$LIMA_HOME/_templates/my.yaml` makes it available as `template:my`.
`limactl info` lists each template's resolved location, which is the quickest way to see which
file a name picked up.

## Verify

```bash
limactl template validate ./my.yaml && echo ok
limactl template copy --fill ./my.yaml - | head -n 20
```

Validation catches schema errors. It does not boot anything: a template can validate and still
fail to provision.

## When it breaks

| Signature | Cause | Fix |
|-----------|-------|-----|
| `template:my` is not found | it is not on any search path | put it in `$LIMA_HOME/_templates`, or pass the file path |
| A `github:` locator fails with 404 | the path or the default branch is wrong | `limactl template url <locator>` to see what it resolved to |
| `github:` fails intermittently | GitHub API rate limit | set `GITHUB_TOKEN` or `GH_TOKEN` |
| A `base:` change has no effect on an existing instance | the instance stores its resolved config | `limactl edit <inst>`, or recreate |
| The template validates but provisioning fails | validation is schema-only | `/var/log/cloud-init-output.log` in the guest |
