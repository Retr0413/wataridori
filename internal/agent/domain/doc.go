// Package domain defines the provider-independent concepts and invariants for
// Wataridori's SRE agent and autonomous recovery loop.
//
// This package owns recovery states, policies, evidence references, action
// types, proposals, authorized plans, quarantine records, and transition
// rules. It must not import model SDKs, cloud clients, storage implementations,
// internal/core, or presentation packages.
package domain
