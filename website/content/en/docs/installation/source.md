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

## Advanced Configuration with Kconfig Tools
(This step is not needed for most users)

To change the build configuration such as the guest architectures, run:

```bash
make config  # For text-based configuration
make menuconfig  # For a menu-based configuration
```

This requires Kconfig tools to be installed. It is also possible to manually edit `.config`. The default configuration can be found in `config.mk` (which follows make syntax).

The tools are available as either `kconfig-frontends` or `kbuild-standalone`. There are two interfaces:
- `conf` for text-based configuration.
- `mconf` for a menu-driven interface.

A Python implementation is available at [Kconfiglib](https://pypi.org/project/kconfiglib). It can be installed with:

```bash
pip install --user kconfiglib
```

This also includes support for `guiconfig` (GUI-based configuration).


