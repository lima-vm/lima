#!/usr/bin/env bash
ARCH=${ARCH:-$(uname -m | sed -e s/arm64/aarch64/)}
echo "CONFIG_GUESTAGENT_OS_LINUX=y"
for arch in x86_64 aarch64 armv7l riscv64; do
  if [ $arch = "$ARCH" ]; then
    x=y
  else
    x=n
  fi
  : ${arch^^}
  echo "CONFIG_GUESTAGENT_ARCH_${_//_}=$x"
done
