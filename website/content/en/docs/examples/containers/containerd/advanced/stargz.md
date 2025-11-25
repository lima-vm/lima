---
title: Accelerating start-up time with eStargz
linkTitle: eStargz
weight: 3
---

[eStargz](https://github.com/containerd/stargz-snapshotter) is an OCI-compatible container image format
that reduces start-up latency using lazy-pulling technique.

The support for eStargz is available by default for `ubuntu-24.04` instances:

```bash
limactl start --name=default template:ubuntu-24.04
```

The latest Ubuntu will be supported too in [a future release](https://github.com/containerd/stargz-snapshotter/issues/2144).

{{% alert title="Hint" color=success %}}
ARM Mac users need to run `limactl start` with `--rosetta` to allow [running AMD64 binaries](../../../../config/multi-arch.md).
This is not an architectural limitation of eStargz, however, Rosetta is needed because the example Python image below
is currently [only available for AMD64](https://github.com/containerd/stargz-snapshotter/issues/2143).
{{% /alert %}}

Without eStargz:

```console
$ time lima nerdctl run --platform=amd64 ghcr.io/stargz-containers/python:3.13-org python3 -c 'print("hi")'
[...]
hi

real	0m23.767s
user	0m0.025s
sys	0m0.020s
```

With eStargz:

```console
$ time lima nerdctl --snapshotter=stargz run --platform=amd64 ghcr.io/stargz-containers/python:3.13-esgz python3 -c 'print("hi")'
[...]
hi

real	0m13.365s
user	0m0.026s
sys	0m0.021s
```

Examples of eStargz images can be found at
<https://github.com/containerd/stargz-snapshotter/blob/main/docs/pre-converted-images.md>.

See also:
- https://github.com/containerd/stargz-snapshotter
- https://github.com/containerd/nerdctl/blob/main/docs/stargz.md