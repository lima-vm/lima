---
title: Accelerating start-up time with eStargz
linkTitle: eStargz
weight: 3
---

[eStargz](https://github.com/containerd/stargz-snapshotter) is an OCI-compatible container image format
that reduces start-up latency using lazy-pulling technique.

The support for eStargz is available by default in Lima.

{{% alert title="Hint" color=success %}}
The example images used below are available for both `amd64` and `arm64`
(see [containerd/stargz-snapshotter#2143](https://github.com/containerd/stargz-snapshotter/issues/2143)).
Rosetta and `--platform=amd64` are not required on ARM Macs; the `--platform=amd64` flag is retained
in the commands below purely so the benchmark numbers remain comparable to earlier measurements.
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
