# GitHub OIDC and Google Cloud setup

GitHub Actions must authenticate to Google Cloud through Workload Identity
Federation. Do not store a service-account JSON key in GitHub.

## Identities

Create separate service accounts:

| Identity | Purpose | Typical access |
|---|---|---|
| dev deployer | plan/apply/status in dev | Cloud Run read/update in dev and Service Account User for the runtime identity |
| status reader | verify dev before a promotion PR | Cloud Run Viewer in dev |
| prod deployer | manual production plan/apply/status | Cloud Run read/update in prod and Service Account User for the runtime identity |

When split-registry support is designed later, use a separate reviewed identity
for Artifact Registry copy. The current automatic promotion PR fails before
copying.

## Trust restrictions

Create Workload Identity Providers that trust GitHub's OIDC issuer and map at
least:

```text
google.subject       = assertion.sub
attribute.repository = assertion.repository
attribute.ref        = assertion.ref
attribute.actor      = assertion.actor
```

Restrict the provider with an attribute condition for the exact manifest
repository. Do not trust every repository under an owner. Use separate
bindings for dev and prod:

- dev: protected default-branch subject/ref only
- prod: the manifest repository plus the `production` GitHub Environment

GitHub Environment jobs normally identify themselves with a subject shaped
like `repo:OWNER/REPOSITORY:environment:production`; branch jobs identify the
ref. Verify the claims emitted by the repository before finalizing the IAM
condition.

Grant each federated principal `roles/iam.workloadIdentityUser` only on its
corresponding service account. Grant Cloud Run and runtime service-account
permissions to that service account, not to the entire workload identity pool.

## Workflow permissions

The caller workflow must declare:

```yaml
permissions:
  contents: read
  id-token: write
```

Reusable workflows cannot elevate permissions omitted by the caller. The
default manual production caller must also use the protected `production` Environment and
must retain `workflow_dispatch` as its only trigger.

The opt-in [merge-approved profile](merge-approved-delivery.md) has a separate
`push` caller and dedicated production identity. Do not broaden the manual
identity's trust to enable it. Restrict the new provider to the exact consumer,
protected default branch, `production` Environment, `push` event and pinned
reusable workflow (`job_workflow_ref`). Map the extra claims if using attribute
bindings, and verify actual emitted claims before activating the provider.
The new deploy identity also needs read access to Dev and the selected artifact;
the proposal status reader needs Prod read access for release comparisons.
Admission validates GitHub protection before OIDC, and read-only deploy preflight
tests effective permissions after OIDC. Neither check replaces the WIF boundary.

## Default manual profile verification

1. Open a dev manifest PR and confirm validation has no OIDC permission.
2. Merge it and confirm only the dev service account is impersonated.
3. Confirm the successful dev run creates a promotion PR but no prod revision.
4. Merge the promotion PR and confirm no prod workflow starts.
5. Run the prod workflow manually from the default branch.
6. Confirm GitHub pauses for Environment approval before the job runs.
7. Approve and verify the resulting Cloud Run revision matches the reviewed
   digest.

For merge-approved acceptance, use the separate-repository checklist in
[merge-approved delivery](merge-approved-delivery.md#release-acceptance), including
denied branch/event/workflow identities and stale plans. A local unit test cannot
prove that the deployed WIF policy enforces these restrictions.
