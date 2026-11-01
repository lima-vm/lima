#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: SUSE LLC
# SPDX-FileCopyrightText: The Rancher Desktop Authors

# Append one row of host resource counters per interval until killed.
#
# The disk columns are cumulative since boot, so the delta between two rows is
# what the runner moved over that window. A bats-app job writes the distro
# image once per VM create, and a per-suite duration cannot separate those
# bytes from CPU cost.
#
# macOS iostat reports one combined figure per device, so only disk_total_bytes
# is filled there; Linux fills reads and writes and leaves the total derived.
#
# Usage: sample.sh <output-dir> [interval-seconds]

set -o errexit -o nounset -o pipefail

output_dir=$1
interval=${2:-5}
mkdir -p "${output_dir}"

samples="${output_dir}/resource-samples.tsv"
processes="${output_dir}/resource-processes.txt"

case "$(uname)" in
    Darwin)
        # The page size differs between Intel and Apple Silicon, so read it
        # rather than assuming 4K, and report memory in bytes.
        page_size=$(vm_stat | sed -n '1s/.*page size of \([0-9]*\).*/\1/p')
        disk=$(iostat -d | head -1 | awk '{print $1}')
        sample_disk() {
            iostat -d -I "${disk}" | tail -1 |
                awk '{printf "%d\t\t\t%.0f", $2, $3 * 1048576}'
        }
        sample_memory() {
            vm_stat | awk -F'[:.]' -v ps="${page_size}" '
                /^Pages free/ {free = $2}
                /^Pageins/ {pi = $2}
                /^Pageouts/ {po = $2}
                /^Swapins/ {si = $2}
                /^Swapouts/ {so = $2}
                END {printf "%d\t%d\t%d\t%d\t%d",
                     free * ps, pi * ps, po * ps, si * ps, so * ps}'
        }
        sample_processes() {
            # macOS top does not report per-process disk counters; pageins
            # and csw are the closest proxies it does report.
            top -l 1 -n 15 -o cpu \
                -stats pid,command,cpu,mem,rsize,pageins,csw,state 2>/dev/null || true
        }
        ;;
    *)
        page_size=$(getconf PAGESIZE)
        sample_disk() {
            # Whole devices only: partition rows would double-count.
            awk '$3 ~ /^(sd[a-z]+|nvme[0-9]+n[0-9]+|vd[a-z]+|xvd[a-z]+)$/ {
                     xfrs += $4 + $8; rd += $6 * 512; wr += $10 * 512
                 }
                 END {printf "%d\t%d\t%d\t%d", xfrs, rd, wr, rd + wr}' /proc/diskstats
        }
        sample_memory() {
            awk -v ps="${page_size}" '
                /^nr_free_pages/ {free = $2}
                /^pgpgin/ {pi = $2 * 1024}
                /^pgpgout/ {po = $2 * 1024}
                /^pswpin/ {si = $2 * ps}
                /^pswpout/ {so = $2 * ps}
                END {printf "%d\t%d\t%d\t%d\t%d", free * ps, pi, po, si, so}' /proc/vmstat
        }
        sample_processes() {
            ps -eo pid,comm,pcpu,rss --sort=-pcpu 2>/dev/null | head -16 || true
        }
        ;;
esac

printf 'epoch_s\tdisk_xfrs\tdisk_read_bytes\tdisk_write_bytes\tdisk_total_bytes\tload1\tmem_free_bytes\tpagein_bytes\tpageout_bytes\tswapin_bytes\tswapout_bytes\tdisk_avail_kb\n' >"${samples}"

tick=0
while :; do
    printf '%s\t%s\t%s\t%s\t%s\n' \
        "$(date +%s)" \
        "$(sample_disk)" \
        "$(uptime | sed 's/.*load averages*: *//' | awk '{print $1}' | tr -d ,)" \
        "$(sample_memory)" \
        "$(df -k / | tail -1 | awk '{print $4}')" \
        >>"${samples}"

    # A process listing costs far more than the counters above, so take one
    # every sixth tick to keep the sampler off the measurement.
    if [[ $((tick % 6)) -eq 0 ]]; then
        {
            echo "=== $(date +%s) ==="
            sample_processes
        } >>"${processes}"
    fi
    tick=$((tick + 1))
    sleep "${interval}"
done
