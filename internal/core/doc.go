// Package core implements the use cases of Wataridori: apply, promote,
// rollback, status and history.
//
// It is called by the CLI in Phase 1 and by the Connect RPC server in
// Phase 2, so request/result types are structured data — presentation
// belongs to the callers. Dependencies on GCP, Git and the store are
// consumed through small interfaces defined in this package.
package core
