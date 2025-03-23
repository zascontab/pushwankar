#!/bin/bash

# Make the script exit on any error
set -e

# Ensure that the required directories exist
mkdir -p pkg/proto

# Install the protoc compiler if it's not already installed
# Uncomment these lines if you need to install protoc
# apt-get update
# apt-get install -y protobuf-compiler

# Install Go plugins for protoc if they're not already installed
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Make sure the newly installed binaries are in your PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Compile the proto files
echo "Compiling proto files..."
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  pkg/proto/*.proto

echo "Proto compilation completed successfully!"

# Now we'll create an empty go file to ensure the package is recognized
if [ ! -f pkg/proto/ensure_package.go ]; then
  cat > pkg/proto/ensure_package.go << EOF
// Package proto contains protobuf definitions and generated code for gRPC.
package proto
EOF
  echo "Created ensure_package.go to ensure pkg/proto is recognized as a Go package."
fi

# We will also fix the go.mod file to ensure it properly references the module
MODULE_NAME=$(grep "^module" go.mod | cut -d ' ' -f 2)
echo "Module name from go.mod: $MODULE_NAME"

# Verify the module is properly referenced in the code
echo "Checking imports in go files..."
grep -r "import.*$MODULE_NAME/pkg/proto" --include="*.go" . || echo "No imports found for $MODULE_NAME/pkg/proto"

# Clear any module cache issues
echo "Cleaning go mod cache..."
go mod tidy

echo "Done! You should now be able to import $MODULE_NAME/pkg/proto in your code."