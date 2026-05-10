---
title: GitHub template URLs
weight: 20
---

| ⚡ Requirement | Lima >= 2.0 |
|----------------|-------------|

Lima provides a special `github:` URL scheme to reference templates from a GitHub repo, as an alternative to using the `https:` scheme with a "raw" URL.

For example the `templates/fedora.yaml` template in the `lima-vm/lima` repo could be referenced as

```
https://raw.githubusercontent.com/lima-vm/lima/refs/heads/master/templates/fedora.yaml
```

Using the `github:` scheme this becomes:

```
github:lima-vm/lima/templates/fedora
```

**⚠️ Note**: `github:` URLs are experimental and the exact semantics may change in future releases.

## General rules

**File extension:**

A `github:` URL without file extension will automatically get a `.yaml` suffix. So the Fedora URL above is the same as

```
github:lima-vm/lima/templates/fedora.yaml
```

**File name:**

The default filename for `github:` URLs is `.lima.yaml`. These URLs all reference the same file:

```
github:lima-vm/lima/.lima.yaml
github:lima-vm/lima/.lima
github:lima-vm/lima/
github:lima-vm/lima
```

**Branch/tag/commit:**

You can append `@TAG` to a `github:` URL to specify a branch, a tag, or a commit id. For example:

```
github:lima-vm/lima/templates/fedora@v2.0.0
```

Lima looks up the default branch of the repo when no `@TAG` is specified. This uses a GitHub API call.

**Note:** Frequent use of `github:` URLs may require setting `GITHUB_TOKEN` or `GH_TOKEN` to a personal access token to avoid GitHub rate limits.

## Testing URL resolution

You can use the `limactl template url` command to see which `https:` URL is generated from a `github:` URL. For example:

```console
❯ limactl template url github:lima-vm/lima/templates/docker
WARN[0000] The github: scheme is still EXPERIMENTAL
https://raw.githubusercontent.com/lima-vm/lima/master/templates/docker.yaml
```

You'll get an error if the template does not exist:

```console
❯ limactl template url github:lima-vm/lima
FATA[0000] file "https://raw.githubusercontent.com/lima-vm/lima/master/.lima.yaml" not found or inaccessible: status 404
```

## Symbolic links

Lima will check if the template file referenced by the `github:` URL is a symlink (or a text file whose content has no spaces, newlines, or colons). In that case it will treat the content as a relative path and return the address of that target file.

For example the `fedora` template is a symlink to `fedora-43.yaml`:

```console
❯ limactl tmpl url github:lima-vm/lima/templates/fedora
https://raw.githubusercontent.com/lima-vm/lima/master/templates/fedora-43.yaml
```

## Org repositories

An "org repo" has identical org and repo names (e.g. `lima-vm/lima-vm`). For these repos, the repo name can be omitted:

```
github:lima-vm/lima-vm/.lima.yaml
github:lima-vm//.lima.yaml
github:lima-vm
```

Org repos support two additional features that enable shorter URLs, even when the main project lives in a different repo (like `lima-vm/lima` instead of `lima-vm/lima-vm`).

**Redirects:**

In an org repo a template file can not only be a symlink, but also a text file containing a `github:` URL. The URL must point to the same GitHub org and must NOT include a `@TAG`. It will be used to replace the original URL.

For example assume the `lima-vm` projects wants to support this URL:

```
github:lima-vm//fedora
```

Then it would have to create a `lima-vm/lima-vm` repo with a `fedora.yaml` file (in the default branch) that contains:

```
github:lima-vm/lima/templates/fedora
```

**Tag propagation:**

Org repo redirects work with tags. For example:

```
github:lima-vm//fedora@v1.2.1
```

Lima resolves this through the following steps:

1. Tries to load `fedora.yaml` from tag `v1.2.1` in the `lima-vm/lima-vm` repo
2. Tag doesn't exist, so falls back to the default branch (`master`)
3. Loads `fedora.yaml@master`, which contains the redirect: `github:lima-vm/lima/templates/fedora`
4. Applies the original tag to the redirect URL

Final resolved URL:

```
github:lima-vm/lima/templates/fedora.yaml@v1.2.1
```

Lima will error if the fallback file doesn't exist or isn't a valid `github:` redirect.
