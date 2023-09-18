---
title: Examples
weight: 3
---
## uname
```console
$ uname -a
Darwin macbook.local 20.4.0 Darwin Kernel Version 20.4.0: Thu Apr 22 21:46:47 PDT 2021; root:xnu-7195.101.2~1/RELEASE_X86_64 x86_64

$ lima uname -a
Linux lima-default 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:12:43 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux

$ LIMA_INSTANCE=arm lima uname -a
Linux lima-arm 5.11.0-16-generic #17-Ubuntu SMP Wed Apr 14 20:10:16 UTC 2021 aarch64 aarch64 aarch64 GNU/Linux
```
{{% fixlinks %}}
See [`./docs/multi-arch.md`](./docs/multi-arch.md) for Intel-on-ARM and ARM-on-Intel .
{{% /fixlinks %}}
## Sharing files across macOS and Linux
```console
$ echo "files under /Users on macOS filesystem are readable from Linux" > some-file

$ lima cat some-file
files under /Users on macOS filesystem are readable from Linux

$ lima sh -c 'echo "/tmp/lima is writable from both macOS and Linux" > /tmp/lima/another-file'

$ cat /tmp/lima/another-file
/tmp/lima is writable from both macOS and Linux
```

## Running containerd containers (compatible with Docker containers)
```console
$ lima nerdctl run -d --name nginx -p 127.0.0.1:8080:80 nginx:alpine
```

> You don't need to run "lima nerdctl" everytime, instead you can use special shortcut called "nerdctl.lima" to do the same thing. By default, it'll be installed along with the lima, so, you don't need to do anything extra. There will be a symlink called nerdctl pointing to nerdctl.lima. This is only created when there is no nerdctl entry in the directory already though. It worths to mention that this is created only via make install. Not included in Homebrew/MacPorts/nix packages.

<http://127.0.0.1:8080> is accessible from both macOS and Linux.

For the usage of containerd and nerdctl (contaiNERD ctl), visit <https://github.com/containerd/containerd> and <https://github.com/containerd/nerdctl>.