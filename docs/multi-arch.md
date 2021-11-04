# Intel-on-ARM and ARM-on-Intel

Lima supports two modes for running Intel-on-ARM and ARM-on-Intel:
- [Slow mode](#slow-mode)
- [Fast mode](#fast-mode)

## [Slow mode: Intel VM on ARM Host / ARM VM on Intel Host](#slow-mode)

Lima can run a VM with a foreign architecture, just by specifying `arch` in the YAML.

```yaml
arch: "x86_64"
# arch: "aarch64"

images:
  - location: "https://cloud-images.ubuntu.com/impish/current/impish-server-cloudimg-amd64.img"
    arch: "x86_64"
  - location: "https://cloud-images.ubuntu.com/impish/current/impish-server-cloudimg-arm64.img"
    arch: "aarch64"

# Disable mounts and containerd, otherwise booting up may timeout if the host is slow
mounts: []
containerd:
  system: false
  user: false
```

Running a VM with a foreign architecture is extremely slow.
Consider using [Fast mode](#fast-mode) whenever possible.

## [Fast mode: Intel containers on ARM VM on ARM Host / ARM containers on Intel VM on Intel Host](#fast-mode)

This mode is significantly faster but often sacrifies compatibility.

Fast mode requires Lima v0.7.3 (nerdctl v0.13.0) or later.

If your VM was created with Lima prior to v0.7.3, you have to recreate the VM with Lima >= 0.7.3,
or upgrade `/usr/local/bin/nerdctl` binary in the VM to [>= 0.13.0](https://github.com/containerd/nerdctl/releases) manually.

Set up:
```bash
lima sudo systemctl start containerd
lima sudo nerdctl run --privileged --rm tonistiigi/binfmt --install all
```

Run containers:
```console
$ lima nerdctl run --platform=amd64 --rm alpine uname -m
x86_64

$ lima nerdctl run --platform=arm64 --rm alpine uname -m
aarch64
```

Build and push container images:
```console
$ lima nerdctl build --platform=amd64,arm64 -t example.com/foo:latest .
$ lima nerdctl push --all-platforms example.com/foo:latest
```

See also https://github.com/containerd/nerdctl/blob/master/docs/multi-platform.md
