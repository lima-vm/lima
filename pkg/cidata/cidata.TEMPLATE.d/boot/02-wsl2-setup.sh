#!/bin/sh
# This script replaces the cloud-init functionality of creating a user and setting its SSH keys
# when using a WSL2 VM.
[ "$LIMA_CIDATA_VMTYPE" = "wsl2" ] || exit 0

# create user
sudo useradd -u "${LIMA_CIDATA_UID}" "${LIMA_CIDATA_USER}" -c "${LIMA_CIDATA_COMMENT}" -d "${LIMA_CIDATA_HOME}"
sudo mkdir "${LIMA_CIDATA_HOME}"/.ssh/
sudo cp "${LIMA_CIDATA_MNT}"/ssh_authorized_keys "${LIMA_CIDATA_HOME}"/.ssh/authorized_keys
sudo chown "${LIMA_CIDATA_USER}" "${LIMA_CIDATA_HOME}"/.ssh/authorized_keys

# add $LIMA_CIDATA_USER to sudoers
echo "${LIMA_CIDATA_USER} ALL=(ALL) NOPASSWD:ALL" | sudo tee -a /etc/sudoers.d/99_lima_sudoers

# symlink CIDATA to the hardcoded path for requirement checks (TODO: make this not hardcoded)
sudo ln -sfFn "${LIMA_CIDATA_MNT}" /mnt/lima-cidata
