---
title: Source Installation
weight: 30
---

## Installing from Source

If you prefer to build Lima from source, follow these steps:

### Prerequisites
Ensure you have the following dependencies installed:
- `git`
- `go`
- `make`

### Build and Install
Run the following commands:

```bash
git clone https://github.com/lima-vm/lima
cd lima
make
sudo make install
```

> **Note:** `sudo make install` is required unless you have write permissions for `/usr/local`. Otherwise, installation may fail.

### Alternative Installation (Without Sudo)
If you prefer installing Lima in your home directory, configure the `PREFIX` and `PATH` as follows:

```bash
make PREFIX=$HOME/.local install
export PATH=$HOME/.local/bin:$PATH
```

### Building External Drivers

> **⚠️ Building drivers as external mode is experimental**

Lima supports building drivers as external executables. For detailed information on creating and building external drivers, see the [Virtual Machine Drivers](../../dev/drivers) guide.

## Packaging Lima for Distribution
After building Lima from source, you may want to package it for installation on other machines:

1. The package for the core component and the native guest agent:
```bash
make clean native
cd _output
tar czf lima-package.tar.gz *
```

2. The package for the additional guest agents:
```
make clean additional-guestagents
cd _output
tar czf lima-additional-guestagents-package.tar.gz *
```

These packages can then be transferred and installed on the target system.
