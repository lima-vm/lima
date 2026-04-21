---
title: Accelerating start-up time with eStargz
linkTitle: eStargz
weight: 3
---

[eStargz](https://github.com/containerd/stargz-snapshotter) is an OCI-compatible container image format
that reduces start-up latency using lazy-pulling technique.

The support for eStargz is available by default in Lima.

Without eStargz:

```console
$ time lima nerdctl run ghcr.io/stargz-containers/python:3.13-org python3 -c 'print("hi")'
[...]
hi

real	0m23.767s
user	0m0.025s
sys	0m0.020s
```

With eStargz:

```console
$ time lima nerdctl --snapshotter=stargz run ghcr.io/stargz-containers/python:3.13-esgz python3 -c 'print("hi")'
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
