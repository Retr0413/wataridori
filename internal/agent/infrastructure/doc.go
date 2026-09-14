// Package infrastructure contains adapters for external systems used by the
// SRE agent and autonomous recovery application.
//
// Adapters may include model providers, GitHub, Cloud Logging, Cloud Run and
// Artifact Registry evidence readers, Git operations, and SQLite or shared
// recovery-state stores. They implement ports owned by the application layer.
// Provider-specific types and credentials must remain inside this layer.
// Infrastructure may translate normalized agent input but must not decide what
// context to inject or authorize recovery actions.
package infrastructure
