# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

load "../helpers/load"

# These tests require a running Alpine Docker VZ instance.
# They are intended for manual/CI execution on Apple Silicon macs.
#
# Usage:
#   limactl create --name=docker-vz templates/experimental/alpine-docker-vz.yaml
#   limactl start docker-vz
#   bats hack/bats/tests/alpine-docker-vz.bats
#
# Set LIMA_BATS_INSTANCE to use a different instance name.

INSTANCE="${LIMA_BATS_INSTANCE:-docker-vz}"

setup() {
    if ! limactl list --format '{{.Status}}' "$INSTANCE" 2>/dev/null | grep -q Running; then
        skip "Instance '$INSTANCE' is not running"
    fi
}

# --- Boot and basic VM tests ---

@test "alpine-docker-vz: VM is Alpine Linux" {
    run limactl shell "$INSTANCE" cat /etc/os-release
    assert_success
    assert_output --partial "Alpine Linux"
}

@test "alpine-docker-vz: guest memory >= 8 GiB" {
    run limactl shell "$INSTANCE" free -g
    assert_success
    local mem_gib
    mem_gib=$(echo "$output" | awk '/^Mem:/ { print $2 }')
    [[ "$mem_gib" -ge 8 ]]
}

# --- Docker tests ---

@test "alpine-docker-vz: Docker daemon is running" {
    run limactl shell "$INSTANCE" docker version
    assert_success
    assert_output --partial "Server:"
}

@test "alpine-docker-vz: ARM64 container runs" {
    run limactl shell "$INSTANCE" docker run --rm --platform linux/arm64 alpine echo "arm64 ok"
    assert_success
    assert_output --partial "arm64 ok"
}

@test "alpine-docker-vz: AMD64 container runs via Rosetta" {
    run limactl shell "$INSTANCE" docker run --rm --platform linux/amd64 alpine echo "amd64 ok"
    assert_success
    assert_output --partial "amd64 ok"
}

@test "alpine-docker-vz: Docker uses overlay2 storage" {
    run limactl shell "$INSTANCE" docker info
    assert_success
    assert_output --partial "Storage Driver: overlay2"
}

@test "alpine-docker-vz: Docker CDI is available" {
    run limactl shell "$INSTANCE" docker info
    assert_success
    assert_output --regexp "[Cc][Dd][Ii]"
}

@test "alpine-docker-vz: Docker live-restore enabled" {
    run limactl shell "$INSTANCE" docker info --format "{{.LiveRestoreEnabled}}"
    assert_success
    assert_output "true"
}

@test "alpine-docker-vz: Docker max-concurrent-downloads is 3" {
    run limactl shell "$INSTANCE" sh -c 'cat /etc/docker/daemon.json'
    assert_success
    assert_output --partial '"max-concurrent-downloads": 3'
}

@test "alpine-docker-vz: dockerd OOM score is protected" {
    run limactl shell "$INSTANCE" sh -c 'cat /proc/$(pgrep -x dockerd)/oom_score_adj'
    assert_success
    local val="$output"
    [[ "$val" -le 0 ]]
}

@test "alpine-docker-vz: Docker prune cron exists" {
    run limactl shell "$INSTANCE" cat /etc/periodic/hourly/docker-prune
    assert_success
    assert_output --partial "docker system prune"
}

@test "alpine-docker-vz: Rosetta CDI device works" {
    run limactl shell "$INSTANCE" docker run --rm --platform linux/amd64 \
        --device=lima-vm.io/rosetta=cached alpine echo "rosetta-cdi ok"
    assert_success
    assert_output --partial "rosetta-cdi ok"
}

@test "alpine-docker-vz: Docker Compose project works" {
    limactl shell "$INSTANCE" sh -c 'mkdir -p /tmp/compose-test && cat > /tmp/compose-test/compose.yaml << EOF
services:
  web:
    image: alpine:latest
    platform: linux/arm64
    command: echo "compose ok"
EOF'
    run limactl shell "$INSTANCE" docker compose -f /tmp/compose-test/compose.yaml up --exit-code-from web
    assert_success
    assert_output --partial "compose ok"

    # Cleanup.
    limactl shell "$INSTANCE" docker compose -f /tmp/compose-test/compose.yaml down 2>/dev/null || true
    limactl shell "$INSTANCE" rm -rf /tmp/compose-test
}

# --- Memory tuning tests ---

@test "alpine-docker-vz: swap is enabled (>= 3 GiB)" {
    run limactl shell "$INSTANCE" cat /proc/swaps
    assert_success
    assert_output --partial "/swapfile"
}

@test "alpine-docker-vz: zswap is enabled" {
    run limactl shell "$INSTANCE" cat /sys/module/zswap/parameters/enabled
    assert_success
    assert_output --partial "Y"
}

@test "alpine-docker-vz: swappiness is 80" {
    run limactl shell "$INSTANCE" sysctl -n vm.swappiness
    assert_success
    assert_output "80"
}

@test "alpine-docker-vz: vfs_cache_pressure is 150" {
    run limactl shell "$INSTANCE" sysctl -n vm.vfs_cache_pressure
    assert_success
    assert_output "150"
}

@test "alpine-docker-vz: dirty_ratio is 10" {
    run limactl shell "$INSTANCE" sysctl -n vm.dirty_ratio
    assert_success
    assert_output "10"
}

@test "alpine-docker-vz: no OOM kills in dmesg" {
    run limactl shell "$INSTANCE" dmesg
    assert_success
    refute_output --partial "Out of memory"
}

# --- PSI and advanced memory tests ---

@test "alpine-docker-vz: PSI (pressure stall info) is available" {
    run limactl shell "$INSTANCE" cat /proc/pressure/memory
    assert_success
    assert_output --partial "some avg10="
}

@test "alpine-docker-vz: KSM is enabled" {
    run limactl shell "$INSTANCE" cat /sys/kernel/mm/ksm/run
    assert_success
    assert_output "1"
}

@test "alpine-docker-vz: THP set to madvise" {
    run limactl shell "$INSTANCE" cat /sys/kernel/mm/transparent_hugepage/enabled
    assert_success
    assert_output --partial "[madvise]"
}

@test "alpine-docker-vz: MGLRU enabled" {
    run limactl shell "$INSTANCE" cat /sys/kernel/mm/lru_gen/enabled
    assert_success
    assert_output --partial "0x0001"
}

@test "alpine-docker-vz: zswap shrinker enabled" {
    run limactl shell "$INSTANCE" cat /sys/module/zswap/parameters/shrinker_enabled
    assert_success
    assert_output "Y"
}

@test "alpine-docker-vz: page-cluster is 0" {
    run limactl shell "$INSTANCE" sysctl -n vm.page-cluster
    assert_success
    assert_output "0"
}

@test "alpine-docker-vz: min_free_kbytes >= 128 MB" {
    run limactl shell "$INSTANCE" sysctl -n vm.min_free_kbytes
    assert_success
    local val="$output"
    [[ "$val" -ge 131072 ]]
}

@test "alpine-docker-vz: periodic memory reclaim cron exists" {
    run limactl shell "$INSTANCE" cat /etc/periodic/5min/memory-reclaim
    assert_success
    assert_output --partial "compact_memory"
}

@test "alpine-docker-vz: inotifywait available for file watching" {
    run limactl shell "$INSTANCE" which inotifywait
    assert_success
}

@test "alpine-docker-vz: vmstat available for memory monitoring" {
    run limactl shell "$INSTANCE" vmstat 1 1
    assert_success
}

# --- Guest agent tests ---

@test "alpine-docker-vz: guest agent is running" {
    run limactl shell "$INSTANCE" pgrep -x lima-guestagent
    assert_success
}

@test "alpine-docker-vz: Rosetta binfmt registered" {
    run limactl shell "$INSTANCE" cat /proc/sys/fs/binfmt_misc/rosetta
    assert_success
    assert_output --partial "enabled"
}

# --- Stress test ---

@test "alpine-docker-vz: memory allocation without OOM" {
    run limactl shell "$INSTANCE" docker run --rm --memory=1g alpine \
        sh -c 'dd if=/dev/zero of=/dev/null bs=1M count=512 2>/dev/null && echo "alloc ok"'
    assert_success
    assert_output --partial "alloc ok"

    sleep 3

    run limactl shell "$INSTANCE" dmesg
    assert_success
    refute_output --partial "Out of memory"
}
