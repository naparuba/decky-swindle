#!/bin/sh
set -e

echo "Container's IP address: `awk 'END{print $1}' /etc/hosts`"

# memscan, the scanning engine, lives in this repository (backend/memscan, with its own
# LICENSE): the plugin and the engine are built from the same commit.
BACKEND_SHARED="memscan.so"
SRC_DIR="/backend/memscan"

mkdir -p /backend/out

echo "Building $BACKEND_SHARED from $SRC_DIR..."
# go.mod may require a newer Go than the image ships; GOTOOLCHAIN=auto fetches it.
cd "$SRC_DIR/cmd/backend"
GOTOOLCHAIN=auto CGO_ENABLED=1 GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o "/backend/out/$BACKEND_SHARED" \
    -buildmode=c-shared

# the cgo header is not needed by the plugin
rm -f "/backend/out/${BACKEND_SHARED%.so}.h"
chmod +x "/backend/out/$BACKEND_SHARED"
