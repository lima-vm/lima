#!/bin/sh
# This script replaces the cloud-init functionality of creating a user and setting its SSH keys
# when using a WSL2 VM.
[ "$LIMA_CIDATA_VMTYPE" = "wsl2" ] || exit 0

# create user
sudo useradd -u "${LIMA_CIDATA_UID}" "${LIMA_CIDATA_USER}" -d "${LIMA_CIDATA_HOME}"
sudo mkdir "${LIMA_CIDATA_HOME}"/.ssh/
sudo cp "${LIMA_CIDATA_MNT}"/ssh_authorized_keys "${LIMA_CIDATA_HOME}"/.ssh/authorized_keys
sudo chown "${LIMA_CIDATA_USER}" "${LIMA_CIDATA_HOME}"/.ssh/authorized_keys

# add $LIMA_CIDATA_USER to sudoers
echo "${LIMA_CIDATA_USER} ALL=(ALL) NOPASSWD:ALL" | sudo tee -a /etc/sudoers.d/99_lima_sudoers

# copy some CIDATA to the hardcoded path for requirement checks (TODO: make this not hardcoded)
sudo mkdir -p /mnt/lima-cidata
sudo cp "${LIMA_CIDATA_MNT}"/meta-data /mnt/lima-cidata/meta-data
