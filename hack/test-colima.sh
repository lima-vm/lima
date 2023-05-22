#!/bin/bash
set -eux -o pipefail

colima start

docker run --rm hello-world

colima stop
