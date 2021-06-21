#!/bin/bash
set -eux -o pipefail

# Create mount points
{{- range $val := .Mounts}}
mkdir -p "{{$val}}"
chown "{{$.User}}" "{{$val}}" || true
{{- end}}

# Install or update the guestagent binary
mkdir -p -m 600 /mnt/lima-cidata
mount -t iso9660 -o ro /dev/disk/by-label/cidata /mnt/lima-cidata
install -m 755 /mnt/lima-cidata/lima-guestagent /usr/local/bin/lima-guestagent
umount /mnt/lima-cidata

# Launch the guestagent service
if [ -f /etc/alpine-release ]; then
  rc-update add lima-guestagent default
  rc-service lima-guestagent start
else
  until [ -e "/run/user/{{.UID}}/systemd/private" ]; do sleep 3; done
  sudo -iu "{{.User}}" "XDG_RUNTIME_DIR=/run/user/{{.UID}}" lima-guestagent install-systemd
fi
