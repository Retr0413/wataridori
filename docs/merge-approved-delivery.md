# Merge-approved delivery

This opt-in Actions profile implements the release contract of #94. The existing
`reusable-prod-apply.yml` and `policy: manual` retain their meanings. No consumer
or production infrastructure is migrated by installing a new CLI.

## Contract

One release PR changes one service image and its JSON plan under
`.wataridori/plans/`. The plan fingerprints the before/after manifest bytes,
environment configuration, delivery configuration, source provenance and Dev
evidence. No workflow, policy or other service changes may accompany it.
Review is on the PR head commit; changing the plan invalidates that review.
Only an image-only service is supported by this profile initially.

The consumer uses a default-branch push to start the dedicated reusable workflow.
It resolves exactly one merged PR, validates its human merge and latest human
reviews, checks effective branch protection and rulesets, and verifies CI on the
artifact's source commit from configured publishers. A PR event, direct push,
unmerged close, missing check, stale evidence or unreadable protection blocks
admission. The human reviewer must have write/maintain/admin permission and must
not be the PR author. The merger must be human with write permission.

Execution is serialized per repository/environment across both delivery profiles.
After waiting for the lock, and again immediately before apply, the reviewed
manifest and plan must still be the newest versions on the default branch.
Unrelated default-branch updates are allowed; a newer release supersedes old runs.
The checkout is pinned to the admitted merge commit, never to a moving branch.
Dev and registry checks are repeated using the selected image. No rebuilding or
substitution with latest Dev is allowed. Rollback is an explicit plan kind with
its own human-reviewed target, not an exception that deploys without approval.

The execution identity needs Cloud Run read/update and artifact read permissions.
GitHub admission is performed before obtaining GCP credentials. Read-only doctor
reports GitHub protection findings; OIDC/WIF and deploy IAM require the separate
credential/preflight and real-environment acceptance checks. Doctor does not
pretend that configuration strings prove effective GCP permissions.

`wataridori delivery preflight --env prod --service api --json` checks
`run.services.get`, `run.services.update`, `run.revisions.get` and
`iam.serviceAccounts.actAs` on the actual runtime identity. It returns
passed/blocked/unknown without changing IAM. Registry access and Dev observation
are verified separately using the selected image. A successful OIDC exchange and
preflight are necessary checks, not a promise that IAM cannot subsequently change.

## Adoption

1. Keep `policy: manual` and set the target service to `applyMode: image-only`.
   Start with one service per PR and manifests at the Git repository root.
   Split-registry image copying and full service replacement are not supported
   by this new adapter. Existing CLI/manual functionality remains available.
   Cloud Run traffic must follow the latest revision at 100%. Image-only apply
   preserves traffic configuration: a pinned old revision or split traffic will
   fail post-deploy verification, not be silently rerouted. Reconcile emergency
   traffic rollback through a separately reviewed operation before normal delivery.
2. Copy [delivery.json](../examples/github-actions/delivery.json) to
   `.wataridori/delivery.json`. Replace the example source repository, workflow
   numeric ID, check names and publisher App IDs with observed CI values.
   Both source-commit checks and PR-head checks are required. Set the maximum age
   (60..86400 seconds) and Dev/Prod relative HTTP paths. HTTP checks currently
   expect 200 and do not authenticate to private endpoints; unsupported endpoints
   block verification instead of being silently skipped.
3. Protect the default branch: at least one human review, stale-review dismissal,
   strict mandatory PR-head checks with pinned publishers, no force pushes,
   deletion or bypass (including admins). Classic protection or an effective
   non-bypass ruleset with these controls is accepted. Unreadable API responses
   return unknown. Environment required reviewers are optional for this profile
   when the PR boundary is enforceable; retaining them intentionally adds another
   approval. The Environment still restricts deployment branches and OIDC scope.
4. Call `reusable-promotion-pr.yml` with `delivery_profile: merge-approved`, one
   `service`, and read-only admission App credentials (or an externally minted
   `admission_token`) able to read source Actions/checks. Explicit provenance must
   identify the completed build/Dev workflow, not the still-running proposal job.
   The usual
   proposal App token still has Contents/Pull requests write only. The status GCP
   identity now needs read access to Prod as well as Dev to describe the currently
   serving release. A plan and fingerprint are committed alongside the image.
5. Copy [deploy-prod-merge.yml](../examples/github-actions/same-repository/deploy-prod-merge.yml)
   to the consumer. Adjust the branch and exact service/plan paths. Replace both
   zero-SHA placeholders with the same reviewed Wataridori commit. The runtime
   checks the Actions run's `referenced_workflows` against the CLI pin. Reserve
   `_wataridori-tool/` for the trusted checkout.
6. Set `admission_app_id` and secret `admission_app_private_key` to mint a fresh
   token inside each run, separately from the proposal App. Grant read-only
   Administration (protection/rulesets), Contents, Actions, Checks and Pull requests
   on the manifest/source repositories. The production workflow defaults to only
   the consumer repository; for a separate same-owner source repository, set
   `admission_repositories` to both repository names (newline separated).
   Cross-owner access requires an externally minted `admission_token` with access
   to both repositories. Organization/enterprise rulesets may need
   additional read visibility; if unavailable admission is blocked. Installation
   tokens expire: the workflow mints and revokes its App token per run. Do not commit tokens or put a one-hour
   token in a permanent repository secret and expect future runs to work.
7. Use a dedicated prod deploy identity with read access to Dev/registry. Update
   WIF conditions for the opt-in caller's `push` event, protected default branch,
   `production` Environment and exact reusable workflow identity. Do not broaden
   the old manual-only identity in place. Coordinate with
   [OIDC setup](github-oidc-setup.md) and test denied paths before activation.

The existing manual workflow shares the repository/environment concurrency group.
Manual, merge-approved and external deployers must not race. Admission allows
unrelated default-branch changes; newer plans, services or config supersede old
runs. No new candidate cancels a running GCP update. A duplicate whose exact image
is already healthy is a no-op.

To diagnose GitHub settings locally, run in the consumer checkout with its pinned
CLI on PATH and `GH_TOKEN` supplied through your credential mechanism:

```sh
ENVIRONMENT=prod SERVICE=api GITHUB_REPOSITORY=owner/repo \
  node /path/to/pinned-wataridori/actions/delivery/run.mjs doctor
```

JSON goes to stdout; Actions also receives a Markdown summary. Settings are not
changed. To disable post-merge execution, disable the consumer merge workflow and
remove the opt-in config through review, then return to manual dispatch. Check
in-flight runs first; configuration changes cannot undo an already-started update.

## Results and recovery

Plans, admission and execution distinguish `proposal_ready`,
`merged_waiting_apply`, `deploying`, `verified`, `blocked`, `failed` and
`superseded`. Actual digest/revision, observation time, PR and Actions links are
reported separately from desired state. Only post-apply digest/Ready/100% traffic
and configured HTTP success mean `verified`. A notification failure does not
mean the deployment failed. A lost runner may leave an in-progress record; inspect
actual state before retrying. Out-of-band deployers must share the lock or be
disabled; GitHub concurrency cannot lock arbitrary GCP clients.
The waiting Check is published after admission inside the job; before a runner
starts (including optional Environment approval), GitHub's run page is the source
of queued/waiting status. Initial admission failures before a valid plan is
identified are reported in the Actions summary, not as an invented PR result.

Rollback proposals reference a previous verified release, retain Git as desired
state and require human approval. Existing traffic rollback remains a separate
emergency operation; reconcile Git afterwards. Database migrations may require
additional recovery steps; an image rollback alone cannot guarantee recovery.

Call `reusable-rollback-pr.yml` only from a manual `workflow_dispatch` caller, with
`verified_release_pr` identifying the intended previously successful release.
See the [rollback caller example](../examples/github-actions/same-repository/rollback-prod.yml).
It checks the previous fingerprint, delivery Check publisher and successful pinned
Actions run, and opens an image-only recovery PR. It has no GCP credentials.
An unknown recovery target is blocked. Duplicate requests for the same run do not
overwrite the existing branch. If a push succeeded but PR creation failed, open a
PR from the reported branch. Review/merge applies the rollback plan through the
same admission workflow, without requiring the old image to still run in Dev.

Optional `delivery_webhook` is a consumer secret containing an HTTPS endpoint.
Payloads contain state, fingerprint, merge commit, selected image, observation and
run URL. A stable `Idempotency-Key` is sent; consumers must deduplicate on it to
cover a runner dying between delivery and recording the sent marker. Ordinary
retries are suppressed via the PR comment marker. Notification errors are
separate and do not reapply. A superseded retry does not erase an existing
successful historical delivery Check.

## Release acceptance

Unit/contract tests use isolated repositories and fake GitHub/Cloud Run responses.
Real acceptance must run in a separate consumer repository and dedicated Cloud
Run service under #74, including both profiles, denied authorization, stale runs,
partial apply and rollback. Changes to BrickLog and its WIF/branch settings are
separate reviewed adoption work. Do not enable this profile before that evidence
exists. A PR with local tests is not a claim of production acceptance.
