// Package cli implements the cobra command layer.
//
// This layer stays thin: it parses flags into core request types and
// renders core result types (as tables or JSON). Business logic lives
// in internal/core.
package cli
