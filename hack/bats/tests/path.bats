# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

INSTANCE=bats

@test "The guest home is accessible via both .guest and .linux paths" {
    limactl shell "$INSTANCE" -- ls -ld /home/"${USER}.guest/.ssh"
    limactl shell "$INSTANCE" -- ls -ld /home/"${USER}.linux/.ssh"
}
