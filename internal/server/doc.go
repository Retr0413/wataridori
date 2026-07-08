// Package server exposes the core use cases over Connect RPC. It is a thin
// adapter: it converts proto messages to core Request types, delegates to a
// core.Engine (built per request so it picks up manifest and Cloud Run state
// freshly), and converts the results back to proto. Rendering lives in the
// clients (CLI, Web UI); this layer only serializes.
//
// See docs/architecture.md ("server → core" dependency) and proto/.
package server
