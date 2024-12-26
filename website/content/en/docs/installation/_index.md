---
title: Installation
weight: 1
---
Supported host OS:
- macOS (the latest version is recommended)
- Linux
- NetBSD (untested)
- Windows (untested)

Prerequisite:
- QEMU 7.0 or later (Required, only if QEMU driver is used)

{{< tabpane text=true >}}

{{% tab header="Homebrew" %}}
```bash
brew install lima
```

Homebrew formula is available [here](https://github.com/Homebrew/homebrew-core/blob/master/Formula/l/lima.rb).
Supports macOS and Linux.
{{% /tab %}}

{{% tab header="MacPorts" %}}
```bash
sudo port install lima
```

Port: <https://ports.macports.org/port/lima/>
{{% /tab %}}

{{% tab header="Nix" %}}
```bash
nix-env -i lima
```

Nix file: <https://github.com/NixOS/nixpkgs/blob/master/pkgs/applications/virtualization/lima/default.nix>
{{% /tab %}}

{{% tab header="Binary" %}}
Download the binary archive of Lima from <https://github.com/lima-vm/lima/releases>, 
and extract it under `/usr/local` (or somewhere else). 

```bash
VERSION=$(curl -fsSL https://api.github.com/repos/lima-vm/lima/releases/latest | jq -r .tag_name)
curl -fsSL "https://github.com/lima-vm/lima/releases/download/${VERSION}/lima-${VERSION:1}-$(uname -s)-$(uname -m).tar.gz" | tar Cxzvm /usr/local
```
{{% /tab %}}

{{% tab header="Source" %}}
The source code can be found at <https://github.com/lima-vm/lima.git>

```bash
git clone https://github.com/lima-vm/lima.git
cd lima
make
make install
```

The installation directory (`$PREFIX`) defaults to `/usr/local`.

#### Advanced configuration with Kconfig tools
(This step is not needed for most users)

To change the build configuration such as the guest architectures, run `make config` or `make menuconfig`.

This requires kconfig tools installed, it is also possible to edit `.config`.
The default configuration can be found in the file `config.mk` (make syntax).

The tools are available as either "kconfig-frontends" or "kbuild-standalone".
There is one `conf` for the text, and one `mconf` for the menu interface.

A python implementation is available at <https://pypi.org/project/kconfiglib>.
It can be installed with `pip install --user kconfiglib`, including `guiconfig`.
{{% /tab %}}
{{< /tabpane >}}
