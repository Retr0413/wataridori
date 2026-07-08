//go:build tools

// Package tools pins the versions of code-generation plugins so that
// `make tools` installs the same versions the committed gen/ code was built
// with. It is never compiled into the binary (the `tools` build tag).
package tools

import (
	_ "connectrpc.com/connect/cmd/protoc-gen-connect-go"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
