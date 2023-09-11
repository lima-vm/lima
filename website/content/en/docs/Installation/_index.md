---
title: Installation
weight: 1
---
> **NOTE**
> Lima is not regularly tested on ARM Mac (due to lack of CI).

## Package Manager

### Homebrew

Homebrew can be used to install lima on macOS and Linux.

```console
brew install lima
```

[Homebrew package](https://github.com/Homebrew/homebrew-core/blob/master/Formula/l/lima.rb) is available here.

## Manual installation

### Prerequisite

- QEMU 7.0 or later (Required, only if QEMU driver is used)

### Install Lima from binary
Download the binary archive of Lima from <https://github.com/lima-vm/lima/releases>, 
and extract it under `/usr/local` (or somewhere else). 

```bash
brew install jq
VERSION=$(curl -fsSL https://api.github.com/repos/lima-vm/lima/releases/latest | jq -r .tag_name)
curl -fsSL "https://github.com/lima-vm/lima/releases/download/${VERSION}/lima-${VERSION:1}-$(uname -s)-$(uname -m).tar.gz" | tar Cxzvm /usr/local
```

### Install Lima from source

To install Lima from the source, run `make && make install`.