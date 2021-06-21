#!/bin/bash
set -eux -o pipefail

# Create mount points
{{- range $val := .Mounts}}
mkdir -p "{{$val}}"
chown "{{$.User}}" "{{$val}}" || true
{{- end}}

# Install or update the guestagent binary
install -m 755 "${LIMA_CIDATA_MNT}"/lima-guestagent /usr/local/bin/lima-guestagent

# Launch the guestagent service
if [ -f /etc/alpine-release ]; then
  rc-update add lima-guestagent default
  rc-service lima-guestagent start
else
  until [ -e "/run/user/{{.UID}}/systemd/private" ]; do sleep 3; done
  sudo -iu "{{.User}}" "XDG_RUNTIME_DIR=/run/user/{{.UID}}" lima-guestagent install-systemd
fi
