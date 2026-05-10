---
title: URL handler plugins
weight: 3
---

> **Warning**
> Support for URL handler plugins is experimental

| ⚡ Requirement | Lima >= 2.0 |
|----------------|-------------|

Lima's template locator supports custom URL schemes through plugins. A plugin named `limactl-url-<scheme>` handles URLs that begin with `<scheme>:`. This lets you create short, memorable template locators for your own workflows.

## How it works

When Lima encounters a URL with an unrecognized scheme (e.g. `dev:webapp`), it:

1. Searches for an executable named `limactl-url-dev` using the standard [plugin discovery](../cli/#plugin-discovery) mechanism
2. Calls the plugin with the part after the colon as its sole argument (in this case, `webapp`)
3. Reads the plugin's stdout, which must be either a URL (with any supported scheme) or a local file path

The plugin's output can itself use a custom scheme. Lima resolves the chain until it reaches a final `https:` URL or local file path. It detects and rejects redirect loops.

If the plugin exits with a non-zero status, Lima reports the error. Any stderr output from the plugin is included in the error message.

You can use `limactl template url` to see what a custom URL resolves to without fetching the template:

```console
$ limactl template url dev:webapp
https://github.example.com/raw/infra/lima-templates/master/webapp.yaml
```

## Creating a URL handler

Create an executable script named `limactl-url-<scheme>` and place it in your `PATH`.

### Returning a URL

The simplest handler maps a short name to a full URL. Here two schemes point at the same repository but select different branches:

**`limactl-url-dev`:**
```bash
#!/bin/sh
echo "https://github.example.com/raw/infra/lima-templates/master/$1.yaml"
```

**`limactl-url-prod`:**
```bash
#!/bin/sh
echo "https://github.example.com/raw/infra/lima-templates/v1.8.3/$1.yaml"
```

```console
$ limactl start dev:webapp    # uses master branch
$ limactl start prod:webapp   # uses pinned release
```

A handler can also generate a pre-signed URL. This `s3:` handler serves templates from a private S3 bucket:

**`limactl-url-s3`:**
```bash
#!/bin/sh
aws s3 presign "s3://my-lima-templates/$1.yaml"
```

```console
$ limactl start s3:webapp
```

### Returning a file path

A handler can return a local file path instead of a URL. This `instance:` handler retrieves the saved configuration of an existing instance:

**`limactl-url-instance`:**
```bash
#!/bin/sh
echo "${LIMA_HOME:-$HOME/.lima}/$1/lima.yaml"
```

This lets you create a new instance using the same template as an existing one:

```console
$ limactl create --name another instance:default
```

### Generating YAML on the fly

A handler can generate a template file at runtime and return its path. This is useful when image URLs contain dynamic components like dates or version numbers that cannot be expressed in static YAML.

**`limactl-url-nightly`:**
```bash
#!/bin/sh
set -eu
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/lima/limactl-url-nightly"
mkdir -p "${CACHE_DIR}"

BUILD=$(curl -fsSL "https://builds.example.com/latest-id")
FILE="${CACHE_DIR}/nightly.yaml"
cat <<EOF >"${FILE}"
images:
- location: "https://builds.example.com/${BUILD}/image-amd64.qcow2"
  arch: "x86_64"
- location: "https://builds.example.com/${BUILD}/image-arm64.qcow2"
  arch: "aarch64"
EOF
echo "$FILE"
```

The generated file stays in the cache directory. It will be overwritten on the next invocation or cleaned up with the rest of the cache.

A template can reference this handler via the `base:` field:

```yaml
base:
- nightly:images
- template:_default/mounts
```

## Composing schemes

Handlers can call `limactl template url` to resolve other schemes, including [`github:`](../../templates/github/). This lets a handler build on existing schemes rather than constructing raw URLs itself.

### Track the latest release

This handler resolves a `github:` URL and then replaces the branch with the latest semver tag:

**`limactl-url-latest`:**
```bash
#!/bin/sh
# Resolve the github: scheme to an https://raw.githubusercontent.com URL
url=$(limactl template url "github:$1")
# Extract "org/repo" from the URL (fields 4-5 of the path)
repo=$(echo "$url" | cut -d'/' -f4-5)
# Find the latest semver release tag (e.g. "v2.1.0"), ignoring pre-releases
tag=$(gh release list --repo "$repo" --json tagName \
         --jq 'map(select(.tagName | test("^v[0-9]+\\.[0-9]+\\.[0-9]+$"))) | .[0].tagName')
# Replace the branch/tag segment in the URL with the release tag
echo "$url" | sed -E "s|(https://raw\.githubusercontent\.com/[^/]+/[^/]+/)[^/]+/|\1$tag/|"
```

```console
$ limactl template url latest:lima-vm/lima/templates/default
https://raw.githubusercontent.com/lima-vm/lima/v2.1.0/templates/default.yaml
```

### Search multiple repos

This handler tries several repositories in order and returns the first match:

**`limactl-url-my`:**
```bash
#!/bin/sh
template=$1
for repo in \
    "github:my-org/templates/%s@main" \
    "github:lima-vm/lima/templates/%s@master"; do
    url=$(limactl template url "$(printf "$repo" "$template")")
    if curl --head --silent --fail "$url" >/dev/null; then
        echo "$url"
        exit
    fi
done
echo "Template $template not found" >&2
exit 1
```

```console
$ limactl start my:custom-distro   # checks my-org/templates first, then lima-vm/lima
```
