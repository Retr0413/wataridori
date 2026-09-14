// Package presentation exposes SRE agent and autonomous recovery use cases to
// Wataridori entry points such as the CLI, reusable GitHub Actions workflows,
// the controller, and Connect RPC.
//
// This layer converts transport input into application commands and renders
// structured results. It must not assemble model context, contain recovery
// policy decisions, or call model and production APIs directly.
package presentation
