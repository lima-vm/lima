---
title: Accelerating start-up time with eStargz
linkTitle: eStargz
weight: 3
---

[eStargz](https://github.com/containerd/stargz-snapshotter) is an OCI-compatible container image format
that reduces start-up latency using lazy-pulling technique.

The support for eStargz is available by default in Lima.

The timings below were measured on an Apple M5 Max (macOS, VZ-backend Lima, default template) pulling the native arm64 images. Numbers are a median of three cold runs (image removed with `nerdctl rmi` between each run).

Without eStargz:

```console
$ time lima nerdctl run ghcr.io/stargz-containers/python:3.13-org python3 -c 'print("hi")'
hi

real	0m14.031s
user	0m0.017s
sys	0m0.018s
```

With eStargz:

```console
$ time lima nerdctl --snapshotter=stargz run ghcr.io/stargz-containers/python:3.13-esgz python3 -c 'print("hi")'
hi

real	0m3.275s
user	0m0.017s
sys	0m0.016s
```

Examples of eStargz images can be found at
<https://github.com/containerd/stargz-snapshotter/blob/main/docs/pre-converted-images.md>.

See also:
- https://github.com/containerd/stargz-snapshotter
- https://github.com/containerd/nerdctl/blob/main/docs/stargz.md
