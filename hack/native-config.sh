#!/usr/bin/env bash
file="$1"
test -e "$file" || exit 1
script=""
ARCH=${ARCH:-$(uname -m | sed -e s/arm64/aarch64/)}
for arch in x86_64 aarch64 armv7l riscv64; do
	if [ $arch = "$ARCH" ]; then
		x=y
	else
		x=n
	fi
	: ${arch^^}
	config="CONFIG_GUESTAGENT_ARCH_${_//_/}"
	script="$script;s/$config=./$config=$x/"
done
sed -e "$script" "$file" >"$file.new" && mv "$file.new" "$file"
