---
title: Installation
weight: 1
---
Supported host OS:
- macOS (the latest version is recommended)
- Linux
- NetBSD (untested)
- DragonFlyBSD (untested)
- Windows (untested)

Prerequisite:
- QEMU (Required, only if [QEMU]({{< ref "/docs/config/vmtype#qemu" >}}) driver is used)

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
{{< /tabpane >}}
