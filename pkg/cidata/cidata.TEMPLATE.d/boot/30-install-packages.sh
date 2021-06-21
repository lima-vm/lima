#!/bin/bash
set -eux -o pipefail

# Install minimum dependencies
if command -v apt-get 2>&1 >/dev/null; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  {{- if .Mounts}}
  apt-get install -y sshfs
  {{- end }}
  {{- if or .Containerd.System .Containerd.User }}
  apt-get install -y iptables
  {{- end }}
  {{- if .Containerd.User}}
  apt-get install -y uidmap fuse3 dbus-user-session
  {{- end }}
elif command -v dnf 2>&1 >/dev/null; then
  : {{/* make sure the "elif" block is never empty */}}
  {{- if .Mounts}}
  dnf install -y fuse-sshfs
  {{- end}}
  {{- if or .Containerd.System .Containerd.User }}
  dnf install -y iptables
  {{- end }}
  {{- if .Containerd.User}}
  dnf install -y shadow-utils fuse3
  if [ ! -f /usr/bin/fusermount ]; then
    # Workaround for https://github.com/containerd/stargz-snapshotter/issues/340
    ln -s fusermount3 /usr/bin/fusermount
  fi
  {{- end}}
elif command -v apk 2>&1 >/dev/null; then
  : {{/* make sure the "elif" block is never empty */}}
  {{- if .Mounts}}
  if ! command -v sshfs 2>&1 >/dev/null; then
    apk update
    apk add sshfs
  fi
  modprobe fuse
  {{- end}}
fi
# Modify /etc/fuse.conf to allow "-o allow_root"
{{- if .Mounts }}
if ! grep -q "^user_allow_other" /etc/fuse.conf ; then
  echo "user_allow_other" >> /etc/fuse.conf
fi
{{- end}}
