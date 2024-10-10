#!/bin/bash

set -eux -o pipefail

: "${RAMDISK_SIZE_IN_GB:=7}"
: "${LIMA_HOME:=$HOME/.lima}"

RAMDISK_SIZE_IN_SECTORS=$((RAMDISK_SIZE_IN_GB * 1024 * 1024 * 1024 / 512))
# hdiutil space-pads the output; strip it.
DISK=$(hdiutil attach -nomount "ram://$RAMDISK_SIZE_IN_SECTORS" | xargs echo)
newfs_hfs "$DISK"
mkdir -p "$LIMA_HOME"
mount -t hfs "$DISK" "$LIMA_HOME"
