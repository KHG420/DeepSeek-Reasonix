#!/bin/bash
# Reasonix wrapper — sets REASONIX_HOME to the workspace's writable .reasonix/
# This is needed because the runtime sandbox makes ~/.reasonix/ read-only.
# Build:  go build -o ./reasonix-bin ./cmd/reasonix
DIR="$(cd "$(dirname "$0")" && pwd)"
export REASONIX_HOME="$DIR/.reasonix"
exec "$DIR/reasonix-bin" "$@"
