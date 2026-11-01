#!/usr/bin/env bash

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# DO NOT MERGE. Samples the host and pushes the samples and VM logs to the
# vmnet-evidence branch until the job ends, because a lost runner uploads nothing.
#
# Usage: vmnet-evidence.sh <job-name>

set -o nounset
shopt -s nullglob

name=$1
sampler=$PWD/hack/vmnet-sample.sh
cd "$HOME" || exit 1
mkdir -p vmnet-evidence/vm
nohup "$sampler" vmnet-evidence 5 >vmnet-evidence/sampler.log 2>&1 &

sha=
while :; do
    # Keep a copy of these logs, because teardown deletes the instances.
    for dir in .lima-bats/*/; do
        instance=$(basename "$dir")
        mkdir -p "vmnet-evidence/vm/${instance}"
        for log in "$dir"serial*.log "$dir"ha.stderr*.log "$dir"*socket_vmnet*.log; do
            cp "$log" "vmnet-evidence/vm/${instance}/"
        done
    done

    tar -czf vmnet-evidence.tgz vmnet-evidence
    base64 <vmnet-evidence.tgz >vmnet-evidence.b64
    jq --null-input --arg message "$name" --arg sha "$sha" --rawfile content vmnet-evidence.b64 \
        '{$message, $content, branch: "vmnet-evidence"} + (if $sha == "" then {} else {$sha} end)' \
        >vmnet-evidence.json
    if new=$(gh api --method PUT "repos/${GITHUB_REPOSITORY}/contents/evidence/${name}.tgz" \
        --input vmnet-evidence.json --jq .content.sha); then
        sha=$new
    fi
    sleep 30
done
