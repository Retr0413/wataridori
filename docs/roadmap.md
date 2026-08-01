# Roadmap

Wataridori stays small by finishing release quality before adding advanced
delivery features.

## Implemented foundation

- [x] manifest schema and validation
- [x] digest-pinned Cloud Run apply
- [x] digest promotion with optional Artifact Registry copy
- [x] revision-based rollback
- [x] status, inventory, and drift classification
- [x] local SQLite operation history
- [x] Connect RPC API generated from protobuf
- [x] controller reconcile loop foundation
- [x] embedded React Web UI
- [x] Pipeline, Timeline, Inventory, and Activity views
- [x] promotion and rollback execution from the UI
- [x] Go tests and Playwright E2E coverage
- [x] CI and GoReleaser configuration

## v0.1.0: public CLI release

- [ ] complete the public-repository security and documentation audit
- [ ] run the full dev-to-prod acceptance test on real GCP
- [ ] add PR-based promotion
- [ ] verify GoReleaser snapshot artifacts and the container image
- [ ] publish installation, IAM, troubleshooting, and rollback runbooks
- [ ] tag and publish `v0.1.0`

## v1.0: safe hosted operation

- [ ] wire remote Git clone/fetch into the controller
- [ ] authenticate and authorize Web and RPC requests
- [ ] route remote CLI operations through Connect RPC
- [ ] add shared durable audit and approval storage
- [ ] add production approval gates
- [ ] add Slack and generic webhook notifications
- [ ] complete a multi-principal, multi-instance, real-GCP acceptance test

## After v1.0

- [ ] import unmanaged Cloud Run services into manifests
- [ ] select Artifact Registry digests through a Git commit or PR
- [ ] progressive traffic rollout
- [ ] revision-tag preview URLs
- [ ] smoke-test and metric-driven automatic rollback
- [ ] event-driven development manifest updates

## Release policy

A milestone is complete only when its implementation, tests, documentation, and
real-GCP acceptance evidence agree. A checked design task is not a substitute
for a published, reproducible release.
