#!/bin/bash
set -eux -o pipefail

# This script prepares Alpine for lima; there is nothing in here for other distros
test -f /etc/alpine-release || exit 0

# Configure apk repos
BRANCH=edge
VERSION_ID=$(awk -F= '$1=="VERSION_ID" {print $2}' /etc/os-release)
case ${VERSION_ID} in
*_alpha*|*_beta*) BRANCH=edge;;
*.*.*) BRANCH=v${VERSION_ID%.*};;
esac

for REPO in main community; do
  URL="https://dl-cdn.alpinelinux.org/alpine/${BRANCH}/${REPO}"
  if ! grep -q "^${URL}$" /etc/apk/repositories; then
    echo "${URL}" >> /etc/apk/repositories
  fi
done

# Alpine doesn't use PAM so we need to explicitly allow public key auth
usermod -p '*' "${LIMA_CIDATA_USER}"

# Alpine disables TCP forwarding, which is needed by the lima-guestagent
sed -i 's/AllowTcpForwarding no/AllowTcpForwarding yes/g' /etc/ssh/sshd_config
rc-service sshd reload

# Create directory for the lima-guestagent socket (normally done by systemd)
mkdir -p /run/user/${LIMA_CIDATA_UID}
chown "${LIMA_CIDATA_USER}" /run/user/${LIMA_CIDATA_UID}
chmod 700 /run/user/${LIMA_CIDATA_UID}

# Install the openrc lima-guestagent service script
cat >/etc/init.d/lima-guestagent <<'EOF'
#!/sbin/openrc-run
supervisor=supervise-daemon

name="lima-guestagent"
description="Forward ports to the lima-hostagent"

export XDG_RUNTIME_DIR="/run/user/${LIMA_CIDATA_UID}"
command=/usr/local/bin/lima-guestagent
command_args="daemon"
command_background=true
command_user="${LIMA_CIDATA_USER}:${LIMA_CIDATA_USER}"
pidfile="${XDG_RUNTIME_DIR}/lima-guestagent.pid"
EOF
chmod 755 /etc/init.d/lima-guestagent

# mount /sys/fs/cgroup
rc-service cgroups start

# `limactl stop` tells acpid to powerdown
rc-update add acpid
rc-service acpid start
