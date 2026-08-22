# GitHub Actions CD

Wataridori provides reusable GitHub Actions workflows for repositories that
already build and publish container images. The application CI supplies an
immutable `IMAGE@sha256:...`; Wataridori validates Git desired state, applies
dev, prepares a production promotion pull request, and waits for explicit
human actions before production.

## Safety contract

The production boundary is intentional and must not be weakened in consumer
workflows:

1. a successful dev apply may automatically create or update a promotion PR
2. the workflow never enables auto-merge or calls a merge API
3. PR creation, review, and merge never call prod apply
4. prod apply has a separate `workflow_dispatch`-only entrypoint
5. its job uses a protected GitHub Environment with required reviewers
6. it accepts only the current default-branch commit
7. no reusable workflow exposes `--force`

Wataridori does not build images. Tag-only references are rejected.

## Available automation

| File | Purpose | Mutation |
|---|---|---|
| `actions/setup` | Install a release with checksum verification, or build a pinned source ref | runner only |
| `reusable-validate.yml` | Validate every manifest without Git or GCP access | none |
| `reusable-dev-update-pr.yml` | Put an external digest in an auto-policy environment and create/update a PR | Git only |
| `reusable-dev-deploy.yml` | Plan, apply, and verify dev with OIDC | Cloud Run dev |
| `reusable-promotion-pr.yml` | Verify dev and create/update a prod promotion PR | Git only |
| `reusable-prod-apply.yml` | Manually plan, apply, and verify prod | Cloud Run prod |

Reusable workflow files live directly under `.github/workflows/` in this
repository. A consumer calls them with a full Wataridori commit SHA in
`wataridori_ref`. Pin the reusable workflow `uses:` reference to that same SHA
before production use; the examples use `master` only because this repository
has not published `v0.1.0` yet.

## Repository setup

Copy the entry workflows from
[`examples/github-actions/same-repository`](../examples/github-actions/same-repository)
to `.github/workflows/` in the manifest repository. Change `master` if its
protected default branch has another name.

Configure these repository variables:

| Variable | Meaning |
|---|---|
| `WATARIDORI_SHA` | Full 40-character commit SHA used by `go install` |
| `MANIFEST_REPOSITORY` | Repository name without owner |
| `WATARIDORI_GITHUB_APP_ID` | GitHub App ID |
| `WATARIDORI_DEV_WIF_PROVIDER` | Dev Workload Identity Provider resource |
| `WATARIDORI_DEV_SERVICE_ACCOUNT` | Dev deploy service account |
| `WATARIDORI_STATUS_WIF_PROVIDER` | Provider used to verify dev before proposing prod |
| `WATARIDORI_STATUS_SERVICE_ACCOUNT` | Read-only Cloud Run service account |
| `WATARIDORI_PROD_WIF_PROVIDER` | Production Workload Identity Provider resource |
| `WATARIDORI_PROD_SERVICE_ACCOUNT` | Production deploy service account |

Configure `WATARIDORI_GITHUB_APP_PRIVATE_KEY` as a repository or organization
secret. The GitHub App installation must be restricted to the manifest
repository and needs only:

- Contents: read and write
- Pull requests: read and write
- Metadata: read

The write workflows request only these permissions on each short-lived
installation token. A GitHub App is required instead of relying on a broad PAT
or assuming the caller repository's `GITHUB_TOKEN` can write another
repository.

## Protected production Environment

Create a GitHub Environment named `production` in the manifest repository.

- configure required reviewers
- prevent self-review where the repository policy supports it
- restrict deployment branches to the protected default branch
- store no long-lived Google service-account key

The example `deploy-prod.yml` contains only `workflow_dispatch`. Do not add
`push`, `pull_request`, `schedule`, `workflow_run`, or repository dispatch
triggers. The reusable workflow also checks the original event and rejects a
non-manual caller.

## Google Cloud authentication

Follow [github-oidc-setup.md](github-oidc-setup.md). Dev, read-only status, and
prod should use separate service accounts and narrowly scoped Workload Identity
Federation bindings. The workflows use short-lived Application Default
Credentials created from GitHub's OIDC token.

## Same-repository flow

```text
application build -> deliver immutable digest -> dev update PR
dev update PR merge -> automatic dev apply -> automatic prod proposal PR
prod proposal PR merge -> no deployment
operator Run workflow -> Environment approval -> prod apply
```

The dev update workflow requires the target environment to use `policy: auto`.
It cannot be pointed at a manual prod environment through an input.

## Separate GitOps repository

Use the files in
[`examples/github-actions/separate-gitops-repository`](../examples/github-actions/separate-gitops-repository).
The application repository receives GitHub App permission to propose only the
dev Git change and receives no GCP credentials. All Cloud Run access stays in
the GitOps repository.

## Split registries

The automatic promotion-PR MVP fails closed when `imageCopy` is configured.
Copying into a production registry is a production mutation and needs a
separate reviewed design. Shared-registry promotion is fully supported because
the pull request changes only the target manifest digest.

## Setup Action

For custom workflows, install a published release:

```yaml
- uses: Retr0413/wataridori/actions/setup@FULL_COMMIT_SHA
  with:
    version: v0.1.0
```

Before the first release, a pinned Action ref can build its bundled source:

```yaml
- uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e
  with:
    go-version: 1.26.5
- uses: Retr0413/wataridori/actions/setup@FULL_COMMIT_SHA
  with:
    version: source
```

Release installation downloads the matching GoReleaser archive and refuses to
install it unless its entry in `checksums.txt` matches.

## Failure and retry behavior

- delivering the same digest is a no-op
- one deterministic bot branch and open PR is reused per environment/service
- a new dev digest updates the existing promotion PR
- a human-authored branch is never selected by name
- failed dev apply does not run the dependent promotion job
- failed prod apply remains a failed manual run; retry does not create a new PR
- rollback stays an explicit Wataridori operation and may intentionally leave
  Git drift that operators must reconcile
