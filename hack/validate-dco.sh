#!/bin/bash
set -eux -o pipefail
# Depends on  github.com/vbatts/git-validation"
git-validation -run DCO || { echo >&2 'ERROR: DCO sign must be added (git commit -a -s --amend)'; exit 1; }
