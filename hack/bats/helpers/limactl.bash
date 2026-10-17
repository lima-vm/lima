# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Create a dummy Lima instance for testing purposes. It cannot be started because it doesn't have an actual image.
# This function intentionally doesn't use create/editflags, but modifies the template with yq instead.
create_dummy_instance() {
    local name=$1
    local expr=${2:-}

    # Template does not validate without an image, and the image must point to a file that exists (for clonefile).
    local template="{images: [location: /etc/profile]}"
    if [[ -n $expr ]]; then
        template="$(limactl yq "$expr" <<<"$template")"
    fi
    limactl create --name "$name" - <<<"$template"
}

# Ensure a Lima instance exists. When LIMA_BATS_REUSE_INSTANCE is set, reuse an
# existing running instance. Otherwise delete and recreate it.
# The instance configuration is determined by its name; add a case below for new names.
# Close file handles 3 and 4 so the host agent doesn't block BATS from exiting.
ensure_instance() {
    local instance=$1
    if [[ -n "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        run limactl list --format '{{.Status}}' "$instance"
        [[ $status == 0 ]] && [[ $output == "Running" ]] && return
    fi
    limactl unprotect "$instance" || :
    limactl delete --force "$instance" || :
    case "$instance" in
        bats)          limactl start --yes --name "$instance" template:default 3>&- 4>&- ;;
        bats-nomount)  limactl start --yes --name "$instance" --mount-none template:default 3>&- 4>&- ;;
        bats-dummy)    create_dummy_instance "$instance" '.disk = "1M"' ;;
        *)
            echo "ensure_instance: unknown instance name '$instance'" >&2
            return 1
            ;;
    esac
}

# Delete the given Lima instance unless LIMA_BATS_REUSE_INSTANCE is set.
delete_instance() {
    local instance=$1
    if [[ -z "${LIMA_BATS_REUSE_INSTANCE:-}" ]]; then
        limactl unprotect "$instance" || :
        limactl delete --force "$instance" || :
    fi
}

# Install socat inside the guest of a running instance, unless it is already there.
install_socat_in_guest() {
    local instance=$1
    limactl shell "$instance" -- sh -c '
        command -v socat >/dev/null && exit 0
        if command -v apt-get >/dev/null; then sudo apt-get install -y socat
        elif command -v dnf >/dev/null; then sudo dnf install -y socat
        elif command -v apk >/dev/null; then sudo apk add socat
        elif command -v pacman >/dev/null; then sudo pacman -Sy --noconfirm socat
        elif command -v zypper >/dev/null; then sudo zypper in -y socat
        else echo "do not know how to install socat in the guest" >&2; exit 1
        fi'
}
