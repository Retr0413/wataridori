# GitOps-preserving Cloud Run management

Wataridori is not a general Cloud Run administration console. New management
features must preserve Git as the desired-state source.

## Boundary

```text
Standard:
UI or CLI -> manifest change -> Git commit or PR -> apply -> Cloud Run

Avoid:
UI or CLI -> direct Cloud Run change -> stale Git manifest
```

Rollback is a deliberate exception for incident recovery. It must report the
resulting drift.

## Inventory

Inventory is implemented as a read-only join between configured Cloud Run
projects and Wataridori manifests.

| Field | Meaning |
|---|---|
| project and region | configured Cloud Run location |
| service | physical Cloud Run service |
| managed | whether a manifest declares the logical service |
| desired image | Git reference |
| actual image | serving revision reference |
| revision and traffic | live routing state |
| readiness | Cloud Run condition |
| Console URL | deep link for detailed operations |

Classification:

- manifest and Cloud Run, same digest: in sync
- manifest and Cloud Run, different digest: drift
- manifest only: not deployed
- Cloud Run only: unmanaged

Inventory does not modify Cloud Run or Git.

## Import

Import is planned after v1.0:

1. select an unmanaged service
2. read the Cloud Run v2 representation
3. convert supported fields to a Wataridori manifest
4. warn about unsupported fields
5. show a complete diff
6. create a commit or PR
7. begin reconciliation only after that Git change is accepted

Import must not perform an update against the existing service.

## Managed image update

A future UI may list Artifact Registry digests and prepare a manifest update.

Required controls:

- display immutable digest, creation time, and provenance when available
- keep the target repository path
- show the manifest diff
- create a commit or PR
- require environment policy and approval before production execution

## Break-glass operation

Emergency direct mutation must be a separate, explicitly named capability.

- require elevated authorization
- display that Git will drift
- require a reason
- record actor, before state, after state, and timestamp
- provide a follow-up path to reconcile Git

It must never be the normal image-update button.

## Drift scope

Wataridori focuses on high-value delivery state:

- image and digest
- serving and latest-ready revision
- traffic allocation
- readiness and failure reason
- missing and unmanaged services

It does not attempt to reproduce a complete field-by-field Cloud Run diff.

## Cloud Console links

Logs and metrics remain in Google Cloud. The UI should link to:

- Cloud Run service and revisions
- a scoped Cloud Logging query
- Cloud Monitoring

This keeps Wataridori focused on delivery decisions.
