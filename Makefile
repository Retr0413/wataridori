.PHONY: gen gen-check tools web-deps web-build build test lint

# Code-generation tool versions are pinned in tools.go / go.mod.
BIN := $(shell go env GOPATH)/bin

# gen regenerates the Connect RPC code from proto/: Go into gen/, TypeScript
# into web/src/gen. Requires the Go plugins on PATH and the protobuf-es plugin
# in web/node_modules (see 'make tools').
gen:
	buf lint
	PATH="$(BIN):$$PATH" buf generate

# tools installs the codegen plugins at the versions pinned in go.mod and
# package.json.
tools: web-deps
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go

# gen-check fails if the committed generated code is stale (used in CI).
gen-check: gen
	git diff --exit-code -- gen/ web/src/gen/

web-deps:
	npm --prefix web ci

# web-build refreshes web/dist, which is committed and embedded by web/assets.go
# so that 'go build' never needs node.
web-build:
	npm --prefix web run build

build: web-build
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run
	npm --prefix web run typecheck
