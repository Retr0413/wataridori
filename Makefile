.PHONY: gen gen-check build test lint

# Code-generation tool versions are pinned in tools.go / go.mod.
BIN := $(shell go env GOPATH)/bin

# gen regenerates the Connect RPC Go code from proto/ into gen/.
# Requires protoc-gen-go and protoc-gen-connect-go on PATH (see 'make tools').
gen:
	buf lint
	PATH="$(BIN):$$PATH" buf generate

# tools installs the codegen plugins at the versions pinned in go.mod.
tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go

# gen-check fails if the committed generated code is stale (used in CI).
gen-check: gen
	git diff --exit-code -- gen/

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run
