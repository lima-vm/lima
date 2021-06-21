#!/bin/bash
set -eux -o pipefail

# This script does not work unless systemd is available
command -v systemctl 2>&1 >/dev/null || exit 0

if [ ! -x /usr/local/bin/nerdctl ]; then
  mkdir -p -m 600 /mnt/lima-cidata
  mount -t iso9660 -o ro /dev/disk/by-label/cidata /mnt/lima-cidata
  tar Cxzf /usr/local /mnt/lima-cidata/nerdctl-full.tgz
  umount /mnt/lima-cidata
fi
{{- if .Containerd.System}}
mkdir -p /etc/containerd
cat >"/etc/containerd/config.toml" <<EOF
  version = 2
  [proxy_plugins]
    [proxy_plugins."stargz"]
      type = "snapshot"
      address = "/run/containerd-stargz-grpc/containerd-stargz-grpc.sock"
EOF
systemctl enable --now containerd buildkit stargz-snapshotter
{{- end}}
{{- if .Containerd.User}}
modprobe tap || true
if [ ! -e "/home/{{.User}}.linux/.config/containerd/config.toml" ]; then
  mkdir -p "/home/{{.User}}.linux/.config/containerd"
  cat >"/home/{{.User}}.linux/.config/containerd/config.toml" <<EOF
  version = 2
  [proxy_plugins]
    [proxy_plugins."fuse-overlayfs"]
      type = "snapshot"
      address = "/run/user/{{.UID}}/containerd-fuse-overlayfs.sock"
    [proxy_plugins."stargz"]
      type = "snapshot"
      address = "/run/user/{{.UID}}/containerd-stargz-grpc/containerd-stargz-grpc.sock"
EOF
  chown -R "{{.User}}" "/home/{{.User}}.linux/.config"
fi
selinux=
if command -v selinuxenabled 2>&1 >/dev/null && selinuxenabled; then
  selinux=1
fi
if [ ! -e "/home/{{.User}}}}.linux/.config/systemd/user/containerd.service" ]; then
  until [ -e "/run/user/{{.UID}}/systemd/private" ]; do sleep 3; done
  if [ -n "$selinux" ]; then
    echo "Temporarily disabling SELinux, during installing containerd units"
    setenforce 0
  fi
  sudo -iu "{{.User}}" "XDG_RUNTIME_DIR=/run/user/{{.UID}}" systemctl --user enable --now dbus
  sudo -iu "{{.User}}" "XDG_RUNTIME_DIR=/run/user/{{.UID}}" containerd-rootless-setuptool.sh install
  sudo -iu "{{.User}}" "XDG_RUNTIME_DIR=/run/user/{{.UID}}" containerd-rootless-setuptool.sh install-buildkit
  sudo -iu "{{.User}}" "XDG_RUNTIME_DIR=/run/user/{{.UID}}" containerd-rootless-setuptool.sh install-fuse-overlayfs
  if ! sudo -iu "{{.User}}" "XDG_RUNTIME_DIR=/run/user/{{.UID}}" containerd-rootless-setuptool.sh install-stargz; then
    echo >&2 "WARNING: rootless stargz does not seem supported on this host (kernel older than 5.11?)"
  fi
  if [ -n "$selinux" ]; then
    echo "Restoring SELinux"
    setenforce 1
  fi
fi
{{- end}}
