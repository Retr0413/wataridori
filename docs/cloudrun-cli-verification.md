# Cloud Run acceptance test

This runbook verifies the complete digest-based development-to-production flow
against real Google Cloud resources.

## Last verified result

The full flow succeeded on July 9, 2026:

1. status reported development and production in sync
2. promotion copied the development digest into the production manifest
3. the same digest was copied to the configured production registry
4. production apply created a new serving revision
5. the production endpoint returned the promoted version
6. rollback restored the previous ready revision
7. the endpoint returned the previous version
8. promotion, apply, and rollback appeared in history

A later rerun was blocked because billing had been disabled on the temporary
test project. No production credentials, project IDs, or endpoints are required
to understand or reproduce this test.

## Topology

Use disposable values:

| Resource | Example |
|---|---|
| GCP project | `YOUR_TEST_PROJECT` |
| logical service | `hello` |
| development region | `DEV_REGION` |
| production region | `PROD_REGION` |
| development registry | `DEV_REGION-docker.pkg.dev/YOUR_TEST_PROJECT/dev-images` |
| production registry | `PROD_REGION-docker.pkg.dev/YOUR_TEST_PROJECT/prod-images` |
| manifest checkout | `/path/to/test-manifests` |

Different regions allow the same physical service name in one project. Separate
projects are also valid.

## Prerequisites

- billing enabled
- Cloud Run Admin API enabled
- Artifact Registry API enabled
- two digest-pinned test images with visibly different responses
- ADC with service update, revision read, and registry copy permissions
- a clean Git manifest repository
- a disposable history database

## Commands

```sh
export REPO=/path/to/test-manifests
export DB=/tmp/wataridori-acceptance.db

go run ./cmd/wataridori status \
  --repo "$REPO" \
  --db "$DB"

go run ./cmd/wataridori apply \
  --repo "$REPO" \
  --env dev \
  --db "$DB"

go run ./cmd/wataridori promote \
  --repo "$REPO" \
  --to prod \
  --yes \
  --db "$DB"

go run ./cmd/wataridori apply \
  --repo "$REPO" \
  --env prod \
  --db "$DB"

go run ./cmd/wataridori rollback \
  --repo "$REPO" \
  --env prod \
  --yes \
  --db "$DB"

go run ./cmd/wataridori history \
  --repo "$REPO" \
  --env prod \
  --db "$DB"
```

Verify the development and production HTTP endpoints after apply and rollback.

## Assertions

- every manifest image contains `@sha256:`
- promotion preserves the target repository path
- source and target content digests are identical after any configured copy
- production serves the promoted revision after apply
- production serves the previous ready revision after rollback
- status reports rollback drift when Git still points to the newer digest
- history identifies action, environment, service, digest, and actor

## Failure cases

The test should also cover:

- disabled billing or API
- missing IAM permission
- missing source digest
- target registry copy failure
- revision readiness timeout
- no previous ready revision
- dirty Git checkout

## Cleanup

Delete disposable Cloud Run services, Artifact Registry repositories, test
images, the local database, and the temporary project when applicable. Never
commit generated credentials or Terraform state.

## Release requirement

Run this procedure again before `v0.1.0` and record only anonymized resource
names and the release commit. A historical success does not replace a
release-candidate acceptance run.
