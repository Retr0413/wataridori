// Package controller runs the reconcile loop: on an interval (and on demand
// via a trigger) it reloads the manifest repository and applies the desired
// state of every policy:auto environment to Cloud Run. Manual environments
// are left to explicit promote+apply.
//
// The loop reconciles from the working copy it is given; refreshing that copy
// from a remote (git fetch/pull) is the caller's responsibility via the
// Refresh hook, so this package stays independent of go-git.
//
// See docs/roadmap.md (Phase 2: reconcile loop / auto follow).
package controller
