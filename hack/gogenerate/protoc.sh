#!/bin/sh

# SPDX-FileCopyrightText: Copyright The Lima Authors
# SPDX-License-Identifier: Apache-2.0

# Generate Go code from a .proto file.
# Expected to be called from `//go:generate` directive.

set -eu
if [ "$#" -ne 1 ]; then
	echo >&2 "Usage: $0 FILE"
	exit 1
fi

PROTO="$1"                         ## a.proto
BASE="$(basename "$PROTO" .proto)" ## a
PB_DESC="${BASE}.pb.desc"          ## a.pb.desc
PB_GO="${BASE}.pb.go"              ## a.pb.go
GRPC_PB_GO="${BASE}_grpc.pb.go"    ## a_grpc.pb.go

set -x

protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative "$PROTO" --descriptor_set_out="$PB_DESC"

# -// - protoc             v6.32.0
# +// - protoc version [omitted for reproducibility]
#
# perl is used because `sed -i` is not portable across BSD (macOS) and GNU.
perl -pi -E 's@(^//.*)protoc[[:blank:]]+v[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+@\1protoc version [omitted for reproducibility]@g' "$PB_GO" "$GRPC_PB_GO"
