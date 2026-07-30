# Concepts and glossary

## Desired and actual state

Wataridori compares:

| State | Source |
|---|---|
| Desired | `wataridori.yaml` and `environments/<env>/*.yaml` in Git |
| Actual | Cloud Run services, revisions, traffic, and image digests |
| Activity | Auxiliary history written by Wataridori |

The activity database is not deployment truth.

## GitOps

GitOps means that an auditable Git change describes the state the delivery
system should apply.

In Wataridori:

- promotion updates a target manifest and creates a commit
- apply reconciles a manifest with Cloud Run
- status reports drift between those two sources

An operation that edits Cloud Run without accounting for Git may be valid as an
emergency rollback, but it creates drift that must be resolved explicitly.

## Container image digest

A tag such as `latest` is a mutable name. A digest such as
`sha256:abc...` is derived from image content.

```text
repository/app:latest         mutable
repository/app@sha256:abc...  immutable
```

Wataridori requires digest-pinned deployment references so the production image
is identical to the image verified in development.

## Environment

An environment maps a logical name such as `dev` or `prod` to:

- update policy
- optional promotion source
- GCP project and region
- service manifest directory
- optional target Artifact Registry repository

## Service identity and Cloud Run name

`name` identifies the same logical service across environments.
`cloudRunName` is the physical Cloud Run service name in one environment.

For example, both `my-api-dev` and `my-api-prod` can use logical name `my-api`.
Promotion joins services by logical name.

## Promotion

Promotion copies an image digest from a source environment manifest into a
target environment manifest. It does not deploy Cloud Run.

## Apply

Apply creates or updates Cloud Run from one environment's manifests. It does
not promote between environments.

## Revision

Cloud Run creates an immutable revision for a service configuration. Traffic
determines which revision serves requests.

## Rollback

Rollback routes traffic to a previous ready revision. It changes actual state
immediately and may leave Git pointing to another image.

## Drift

Drift means desired and actual state differ. Wataridori currently focuses on
image digest, serving revision, traffic, readiness, missing services, and
unmanaged services rather than diffing every Cloud Run field.

## Inventory

Inventory joins all services returned by Cloud Run with all services declared
by manifests. It is read-only.

## Timeline and activity

Timeline is reconstructed from Cloud Run revisions. Activity is reconstructed
from Wataridori's history store. Timeline answers what ran; activity answers
what Wataridori did.

## Connect RPC

Connect RPC defines methods and request/response messages with protobuf while
supporting browser-friendly HTTP transports.

Proto is the API schema, not a requirement to use binary payloads. The Connect
protocol may use protobuf JSON or binary protobuf. Generated clients preserve
the same message contract.

## Reconciliation

Reconciliation repeatedly compares desired and actual state and applies changes
for eligible automatic environments. A complete hosted loop also needs to
refresh its Git checkout before loading manifests.

## Application Default Credentials

ADC is Google's standard credential discovery mechanism. Local users commonly
run `gcloud auth application-default login`; hosted deployments should use a
dedicated service account and Workload Identity.

## Source tree

| Path | Responsibility |
|---|---|
| `cmd/wataridori` | process entrypoint |
| `internal/cli` | command and rendering layer |
| `internal/core` | delivery use cases |
| `internal/manifest` | desired-state schema |
| `internal/cloudrun` | actual-state API |
| `internal/registry` | image copy |
| `internal/gitops` | commits and Git synchronization |
| `internal/store` | auxiliary history |
| `internal/server` | Connect API |
| `internal/controller` | reconcile loop |
| `proto` | API contract |
| `web` | embedded React UI |
