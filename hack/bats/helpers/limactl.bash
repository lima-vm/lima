# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Create a dummy Lima instance for testing purposes. It cannot be started because it doesn't have an actual image.
# This function intentionally doesn't use create/editflags, but modifies the template with yq instead.
create_dummy_instance() {
    local name=$1
    local expr=$2

    # Template does not validate without an image, and the image must point to a file that exists (for clonefile).
    local template="{images: [location: /etc/profile]}"
    if [[ -n $expr ]]; then
        template="$(limactl yq "$expr" <<<"$template")"
    fi
    limactl create --name "$name" - <<<"$template"
}
