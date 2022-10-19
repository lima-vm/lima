#!/bin/sh
set -eu
# Simple wrapper for 'lima apptainer run'
exec singularity --quiet run "$@"
