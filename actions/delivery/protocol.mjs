import {createHash} from 'node:crypto';

export const configPath = '.wataridori/delivery.json';
export const sha = /^[a-f0-9]{40}$/;
const image = /^[^\s@]+@sha256:[a-f0-9]{64}$/;
const identity = /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$/;
export const hash = data => createHash('sha256').update(data).digest('hex');
const canonical = value => Array.isArray(value) ? value.map(canonical) : value && typeof value === 'object'
  ? Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])])) : value;
export function fingerprint(plan) {
  const {fingerprint: ignored, ...content} = plan;
  return 'sha256:' + hash(JSON.stringify(canonical(content)));
}
export function requireThat(condition, message) { if (!condition) throw new Error(message); }
export function planPath(env, service) {
  requireThat(identity.test(env) && identity.test(service), 'invalid environment/service identifier');
  return `.wataridori/plans/${env}/${service}.json`;
}
export function safePath(path) {
  return typeof path === 'string' && /^[a-zA-Z0-9_./-]+$/.test(path) && !path.startsWith('/') && !path.split('/').some(p => p === '..' || p === '.' || !p);
}
export function policy(config, env) {
  requireThat(config.version === 1, 'unsupported delivery config version');
  const p = config.environments?.[env];
  requireThat(p?.profile === 'merge-approved', 'environment has not opted into merge-approved');
  requireThat(/^[\w.-]+\/[\w.-]+$/.test(p.sourceRepository), 'sourceRepository is required');
  requireThat(Number.isSafeInteger(p.sourceWorkflowId) && p.sourceWorkflowId > 0, 'sourceWorkflowId is required');
  requireThat(Number.isSafeInteger(p.maxAgeSeconds) && p.maxAgeSeconds >= 60 && p.maxAgeSeconds <= 86400, 'maxAgeSeconds must be 60..86400');
  for (const field of ['requiredChecks', 'prChecks']) {
    requireThat(Array.isArray(p[field]) && p[field].length > 0, `${field} must not be empty`);
    for (const c of p[field]) requireThat(typeof c.name === 'string' && c.name.length > 0 && Number.isSafeInteger(c.appId) && c.appId > 0, `${field} requires check name and appId`);
  }
  for (const field of ['devHTTPPaths', 'prodHTTPPaths']) {
    requireThat(Array.isArray(p[field]) && p[field].length > 0 && p[field].length <= 10, `${field} requires 1..10 paths`);
    for (const path of p[field]) requireThat(typeof path === 'string' && /^\/(?!\/)[^?#\s\\]*$/.test(path), 'health checks must use relative absolute-paths');
  }
  return p;
}
export function validatePlan(plan, p, env, service, now = Date.now(), {allowExpiredObservation = false} = {}) {
  requireThat(plan.version === 1 && ['promotion', 'rollback'].includes(plan.kind), 'unsupported release plan');
  requireThat(plan.environment === env && plan.service === service, 'plan target mismatch');
  planPath(env, service);
  requireThat(plan.fingerprint === fingerprint(plan), 'plan fingerprint mismatch');
  requireThat(image.test(plan.image) && image.test(plan.oldImage) && plan.image !== plan.oldImage, 'plan requires different immutable images');
  requireThat(plan.applyMode === 'image-only' && safePath(plan.serviceFile), 'only image-only service manifests are supported');
  requireThat(plan.serviceFile.endsWith('.yaml') || plan.serviceFile.endsWith('.yml'), 'invalid manifest extension');
  for (const field of ['beforeHash', 'afterHash', 'configHash', 'policyHash']) requireThat(/^[a-f0-9]{64}$/.test(plan[field]), `invalid ${field}`);
  const age = now - Date.parse(plan.createdAt);
  requireThat(Number.isFinite(age) && age >= 0 && (allowExpiredObservation || age <= p.maxAgeSeconds * 1000), 'release evidence is expired or from the future');
  requireThat(plan.source?.sourceRepository === p.sourceRepository && sha.test(plan.source?.sourceCommit), 'untrusted source provenance');
  requireThat(new RegExp(`^https://github\\.com/${p.sourceRepository.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/actions/runs/[0-9]+$`).test(plan.source.workflowRun), 'invalid source workflow URL');
  requireThat(plan.kind !== 'promotion' || identity.test(plan.fromEnvironment), 'invalid source environment');
}

// A classic protection with no bypass is sufficient even if additional rules
// exist. Ruleset-only repositories must expose each effective ruleset's bypasses.
export function protectionFindings(protection, rules, rulesets, p) {
  const findings = [];
  let reviewCount = 0;
  const required = [];
  const r = protection?.required_pull_request_reviews;
  const bypass = r?.bypass_pull_request_allowances;
  if (protection?.enforce_admins?.enabled && r?.dismiss_stale_reviews && r.required_approving_review_count >= 1 &&
      !Object.values(bypass ?? {}).some(v => Array.isArray(v) && v.length) &&
      protection.allow_force_pushes?.enabled === false && protection.allow_deletions?.enabled === false &&
      protection.required_status_checks?.strict) {
    reviewCount = r.required_approving_review_count;
    required.push(...(protection.required_status_checks.checks ?? []).map(c => ({name: c.context, appId: c.app_id})));
  }
  for (const set of rulesets) {
    if (set.enforcement !== 'active' || !Array.isArray(set.bypass_actors) || set.bypass_actors.length) continue;
    const effective = rules.filter(rule => rule.ruleset_id === set.id);
    const pr = effective.find(rule => rule.type === 'pull_request')?.parameters;
    const checks = effective.find(rule => rule.type === 'required_status_checks')?.parameters;
    if (pr?.dismiss_stale_reviews_on_push && pr.required_approving_review_count >= 1 && checks?.strict_required_status_checks_policy &&
        effective.some(rule => rule.type === 'non_fast_forward') && effective.some(rule => rule.type === 'deletion')) {
      reviewCount = Math.max(reviewCount, pr.required_approving_review_count);
      required.push(...checks.required_status_checks.map(c => ({name: c.context, appId: c.integration_id})));
    }
  }
  if (!reviewCount) findings.push('Require enforced reviews, stale-review dismissal, strict checks, no bypass/force push/deletion (including admins).');
  for (const c of p.prChecks) if (!required.some(x => x.name === c.name && x.appId === c.appId)) findings.push(`Require protected PR check ${c.name} from app ${c.appId}.`);
  return {status: findings.length ? 'blocked' : 'passed', findings, reviewCount};
}
export function validateReviews(pr, reviews, writable, count) {
  requireThat(pr.merged && pr.merged_at && pr.merged_by?.type === 'User' && writable.has(pr.merged_by.login), 'PR must be merged by a human with write permission');
  const latest = new Map();
  for (const review of [...reviews].sort((a,b) => a.id - b.id)) {
    if (review.state !== 'COMMENTED' && review.state !== 'PENDING') latest.set(review.user?.login, review);
  }
  let approvals = 0;
  for (const r of latest.values()) {
    if (r.user?.type !== 'User' || !writable.has(r.user.login)) continue;
    requireThat(r.state !== 'CHANGES_REQUESTED', 'outstanding changes requested');
    if (r.state === 'APPROVED' && r.commit_id === pr.head.sha && r.user.login !== pr.user.login &&
        Date.parse(r.submitted_at) <= Date.parse(pr.merged_at)) approvals++;
  }
  requireThat(approvals >= count, 'missing human approval of the latest PR head');
}
export function validateChecks(checks, required, commit, maxAge, now = Date.now()) {
  for (const expected of required) {
    const matching = checks.filter(c => c.name === expected.name && c.app?.id === expected.appId).sort((a,b) => b.id-a.id);
    const c = matching[0];
    const age = now - Date.parse(c?.completed_at);
    requireThat(c?.head_sha === commit && c.status === 'completed' && c.conclusion === 'success' && Number.isFinite(age) && age >= 0 && age <= maxAge*1000,
      `missing, failed, or stale check: ${expected.name} on ${commit}`);
  }
}

export function releaseMarkdown(plan) {
  const escape = text => String(text).replace(/[<>&`|\r\n]/g, ' ');
  const attention = (plan.attention ?? []).map(item => `- ${escape(item)}`).join('\n') || 'No filename-based warnings available. Review compatibility and migrations yourself; this is not a safety guarantee.';
  return `## Wataridori release\n\n- Kind: ${plan.kind}\n- Service: ${escape(plan.service)}\n- Image: \`${plan.oldImage}\` → \`${plan.image}\`\n- Plan: \`${plan.fingerprint}\`\n- Source: ${plan.source.sourceRepository}@${plan.source.sourceCommit}\n- CI: ${plan.source.workflowRun}\n- Evidence observed: ${plan.createdAt}\n\nHuman review and merge authorizes this exact plan. The merge-approved workflow rechecks admission and applicable quality gates before applying. Deployment completes only after production verification.\n\n### Changes since the observed production release\n\n${plan.changes?.length ? plan.changes.map(c => `- ${escape(c.sha)} ${escape(c.message)} ${escape(c.url ?? '')}${(c.prs ?? []).map(pr => ` — ${escape(pr.title)} ${escape(pr.url)}`).join('')}`).join('\n') : plan.comparison === 'complete' ? 'No new source commits since the observed production release.' : 'Source comparison unavailable; inspect the linked source changes before approval.'}\n\n### Attention\n\n${attention}\n\n<details><summary>Evidence (health is not product QA)</summary>\n\n\`\`\`json\n${JSON.stringify(plan.evidence, null, 2).replaceAll('<', '\\u003c')}\n\`\`\`\n</details>\n`;
}
