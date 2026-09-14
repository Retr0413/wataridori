// Package application orchestrates the bounded SRE agent and autonomous
// recovery use cases.
//
// It owns the ports consumed by the use cases, assembles versioned model input,
// selects and redacts evidence, runs the bounded read-only tool loop, validates
// recovery proposals, requests deterministic policy authorization, executes
// authorized plans through narrow core ports, and verifies final convergence.
//
// Business data is passed as explicit typed values. context.Context is used
// only for cancellation, deadlines, and tracing.
package application
