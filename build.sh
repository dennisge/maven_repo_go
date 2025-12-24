#!/bin/bash
set -e

APP_NAME="maven_server"
OUTPUT_DIR="bin"

# Default to host
GOOS=${1:-$(go env GOOS)}
GOARCH=${2:-$(go env GOARCH)}

echo "Building $APP_NAME for $GOOS/$GOARCH..."

# Create platform specific output dir
PLATFORM_DIR="$OUTPUT_DIR/$GOOS-$GOARCH"
mkdir -p "$PLATFORM_DIR"

output_name=$APP_NAME
if [ "$GOOS" = "windows" ]; then
    output_name+=".exe"
fi

env GOOS=$GOOS GOARCH=$GOARCH go build -o "$PLATFORM_DIR/$output_name"

echo "Build successful! Binary is at $PLATFORM_DIR/$output_name"
