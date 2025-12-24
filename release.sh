#!/bin/bash
set -e

APP_NAME="maven_server"
VERSION="1.0.0"
RELEASE_DIR="release"

echo "Creating release for $APP_NAME v$VERSION..."
mkdir -p $RELEASE_DIR

# Defined platforms
platforms=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64")

for platform in "${platforms[@]}"; do
    IFS="/" read -r -a parts <<< "$platform"
    GOOS="${parts[0]}"
    GOARCH="${parts[1]}"

    ./build.sh $GOOS $GOARCH

    archive_name="$APP_NAME-$VERSION-$GOOS-$GOARCH.tar.gz"
    echo "Packaging $archive_name..."

    # Create a temporary directory for packaging to include README, etc.
    # We name the folder inside package just "maven_server" or similar for clean extraction
    package_dir="maven_server-$VERSION"
    rm -rf "$package_dir" # Cleanup from prev loop
    mkdir -p "$package_dir"
    
    # Copy binary
    bin_name=$APP_NAME
    if [ "$GOOS" = "windows" ]; then
        bin_name+=".exe"
    fi
    cp "bin/$GOOS-$GOARCH/$bin_name" "$package_dir/"
    
    # Copy README
    cp README.md "$package_dir/"

    # Compress
    tar -czf "$RELEASE_DIR/$archive_name" "$package_dir"
    
    # Cleanup temp dir
    rm -rf "$package_dir"
done

echo "Releases created in $RELEASE_DIR:"
ls -lh $RELEASE_DIR
