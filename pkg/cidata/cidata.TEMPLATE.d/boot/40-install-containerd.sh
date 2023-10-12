#!/bin/bash
set -eux

if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" != 1 ] && [ "${LIMA_CIDATA_CONTAINERD_USER}" != 1 ]; then
	exit 0
fi

# This script does not work unless systemd is available
command -v systemctl >/dev/null 2>&1 || exit 0

# Extract bin/nerdctl and compare whether it is newer than the current /usr/local/bin/nerdctl (if already exists).
# Takes 4-5 seconds. (FIXME: optimize)
tmp_extract_nerdctl="$(mktemp -d)"
tar Cxzf "${tmp_extract_nerdctl}" "${LIMA_CIDATA_MNT}"/nerdctl-full.tgz bin/nerdctl

if [ ! -f "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/nerdctl ] || [[ "${tmp_extract_nerdctl}"/bin/nerdctl -nt "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/nerdctl ]]; then
	if [ -f "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/nerdctl ]; then
		(
			set +e
			echo "Upgrading existing nerdctl"
			echo "- Old: $("${LIMA_CIDATA_GUEST_INSTALL_PREFIX}"/bin/nerdctl --version)"
			echo "- New: $("${tmp_extract_nerdctl}"/bin/nerdctl --version)"
			systemctl disable --now containerd buildkit stargz-snapshotter
			sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" "PATH=${PATH}" containerd-rootless-setuptool.sh uninstall
		)
	fi
	tar Cxzf "${LIMA_CIDATA_GUEST_INSTALL_PREFIX}" "${LIMA_CIDATA_MNT}"/nerdctl-full.tgz

	mkdir -p /etc/bash_completion.d
	nerdctl completion bash >/etc/bash_completion.d/nerdctl
	# TODO: enable zsh completion too
fi

rm -rf "${tmp_extract_nerdctl}"

: "${CONTAINERD_NAMESPACE:=default}"
# Overridable in .bashrc
: "${CONTAINERD_SNAPSHOTTER:=overlayfs}"

if [ "${LIMA_CIDATA_CONTAINERD_SYSTEM}" = 1 ]; then
	mkdir -p /etc/containerd /etc/buildkit
	cat >"/etc/containerd/config.toml" <<EOF
  version = 2
  [proxy_plugins]
    [proxy_plugins."stargz"]
      type = "snapshot"
      address = "/run/containerd-stargz-grpc/containerd-stargz-grpc.sock"
EOF
	cat >"/etc/buildkit/buildkitd.toml" <<EOF
[worker.oci]
  enabled = false

[worker.containerd]
  enabled = true
  namespace = "${CONTAINERD_NAMESPACE}"
  snapshotter = "${CONTAINERD_SNAPSHOTTER}"
EOF
	systemctl enable --now containerd buildkit stargz-snapshotter
fi

if [ "${LIMA_CIDATA_CONTAINERD_USER}" = 1 ]; then
	if [ ! -e "${LIMA_CIDATA_HOME}/.config/containerd/config.toml" ]; then
		mkdir -p "${LIMA_CIDATA_HOME}/.config/containerd"
		cat >"${LIMA_CIDATA_HOME}/.config/containerd/config.toml" <<EOF
  version = 2
  [proxy_plugins]
    [proxy_plugins."fuse-overlayfs"]
      type = "snapshot"
      address = "/run/user/${LIMA_CIDATA_UID}/containerd-fuse-overlayfs.sock"
    [proxy_plugins."stargz"]
      type = "snapshot"
      address = "/run/user/${LIMA_CIDATA_UID}/containerd-stargz-grpc/containerd-stargz-grpc.sock"
EOF
		chown -R "${LIMA_CIDATA_USER}" "${LIMA_CIDATA_HOME}/.config"
	fi
	selinux=
	if command -v selinuxenabled >/dev/null 2>&1 && selinuxenabled; then
		selinux=1
	fi
	if [ ! -e "/etc/apparmor.d/usr.local.bin.rootlesskit" ] && [ -e "/etc/apparmor.d/abi/4.0" ] && [ -e "/proc/sys/kernel/apparmor_restrict_unprivileged_userns" ]; then
		cat >"/etc/apparmor.d/usr.local.bin.rootlesskit" <<EOF
# Ubuntu 23.10 introduced kernel.apparmor_restrict_unprivileged_userns
# to restrict unsharing user namespaces:
# https://ubuntu.com/blog/ubuntu-23-10-restricted-unprivileged-user-namespaces
#
# kernel.apparmor_restrict_unprivileged_userns is still opt-in in Ubuntu 23.10,
# but it is expected to be enabled in future releases of Ubuntu.
abi <abi/4.0>,
include <tunables/global>

/usr/local/bin/rootlesskit flags=(unconfined) {
  userns,

  # Site-specific additions and overrides. See local/README for details.
  include if exists <local/usr.local.bin.rootlesskit>
}
EOF
		systemctl restart apparmor.service
	fi
	if [ ! -e "${LIMA_CIDATA_HOME}/.config/systemd/user/containerd.service" ]; then
		until [ -e "/run/user/${LIMA_CIDATA_UID}/systemd/private" ]; do sleep 3; done
		if [ -n "$selinux" ]; then
			echo "Temporarily disabling SELinux, during installing containerd units"
			setenforce 0
		fi
		sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" systemctl --user enable --now dbus
		sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" "PATH=${PATH}" containerd-rootless-setuptool.sh install
		sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" "PATH=${PATH}" \
			"CONTAINERD_NAMESPACE=${CONTAINERD_NAMESPACE}" "CONTAINERD_SNAPSHOTTER=${CONTAINERD_SNAPSHOTTER}" \
			containerd-rootless-setuptool.sh install-buildkit-containerd

		# $CONTAINERD_SNAPSHOTTER is configured in 20-rootless-base.sh, when the guest kernel is < 5.13, or the instance was created with Lima < 0.9.0.
		if [ "$(sudo -iu "${LIMA_CIDATA_USER}" sh -ec 'echo $CONTAINERD_SNAPSHOTTER')" = "fuse-overlayfs" ]; then
			sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" "PATH=${PATH}" containerd-rootless-setuptool.sh install-fuse-overlayfs
		fi

		if compare_version.sh "$(uname -r)" -ge "5.13"; then
			sudo -iu "${LIMA_CIDATA_USER}" "XDG_RUNTIME_DIR=/run/user/${LIMA_CIDATA_UID}" "PATH=${PATH}" containerd-rootless-setuptool.sh install-stargz
		else
			echo >&2 "WARNING: the guest kernel seems older than 5.13. Skipping installing rootless stargz."
		fi
		if [ -n "$selinux" ]; then
			echo "Restoring SELinux"
			setenforce 1
		fi
	fi
fi
