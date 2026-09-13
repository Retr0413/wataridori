import {requireThat, protectionFindings, validateReviews, validateChecks} from './protocol.mjs';

export class GitHub {
  constructor(token, fetcher = fetch) { this.token = token; this.fetcher = fetcher; }
  async request(path, method = 'GET', body, allowMissing = false) {
    requireThat(path.startsWith('repos/') || path.startsWith('orgs/'), 'unsupported GitHub API path');
    const response = await this.fetcher(`https://api.github.com/${path}`, {
      method, redirect: 'error', signal: AbortSignal.timeout(30000),
      headers: {Authorization: `Bearer ${this.token}`, Accept: 'application/vnd.github+json', 'Content-Type':'application/json', 'X-GitHub-Api-Version': '2022-11-28'},
      ...(body === undefined ? {} : {body: JSON.stringify(body)}),
    });
    if (allowMissing && response.status === 404) return null;
    requireThat(response.ok, `GitHub ${method} ${path.split('?')[0]}: HTTP ${response.status} (check installation, permissions and plan support)`);
    return response.status === 204 ? null : response.json();
  }
  async list(path, key) {
    const out = [];
    for (let page=1; page<=50; page++) {
      const result = await this.request(`${path}${path.includes('?') ? '&' : '?'}per_page=100&page=${page}`);
      const rows = key ? result[key] : result;
      requireThat(Array.isArray(rows), 'unexpected GitHub list response'); out.push(...rows);
      if (rows.length < 100) return out;
    }
    throw new Error('GitHub pagination limit exceeded; result is incomplete');
  }
}

export async function doctor(api, repo, branch, p) {
  try {
    const protection = await api.request(`repos/${repo}/branches/${encodeURIComponent(branch)}/protection`, 'GET', undefined, true);
    const rules = await api.list(`repos/${repo}/rules/branches/${encodeURIComponent(branch)}`);
    const sets = [];
    for (const id of new Set(rules.map(r => r.ruleset_id))) {
      requireThat(Number.isSafeInteger(id), 'ruleset identity unavailable');
      const set = await api.request(`repos/${repo}/rulesets/${id}?includes_parents=true`);
      requireThat(Array.isArray(set.bypass_actors), 'ruleset bypass visibility unavailable');
      sets.push(set);
    }
    return {...protectionFindings(protection, rules, sets, p), gcp: 'not_checked: run OIDC credential and Cloud Run/registry verification; validate deploy IAM in dedicated acceptance'};
  } catch (error) { return {status: 'unknown', findings: [error.message], reviewCount: 0, gcp: 'not_checked'}; }
}

export async function quality(api, p, source, now = Date.now()) {
  requireThat(source.sourceRepository === p.sourceRepository, 'source repository mismatch');
  const id = source.workflowRun.split('/').at(-1);
  requireThat(/^[0-9]+$/.test(id), 'invalid workflow run ID');
  const run = await api.request(`repos/${p.sourceRepository}/actions/runs/${id}`);
  requireThat(run.repository?.full_name === p.sourceRepository && run.head_repository?.full_name === p.sourceRepository &&
    run.head_sha === source.sourceCommit && run.workflow_id === p.sourceWorkflowId &&
    ['push', 'workflow_dispatch', 'repository_dispatch'].includes(run.event) &&
    run.status === 'completed' && run.conclusion === 'success', 'source workflow identity, commit, or result is not trusted');
  const checks = await api.list(`repos/${p.sourceRepository}/commits/${source.sourceCommit}/check-runs?filter=latest`, 'check_runs');
  validateChecks(checks, p.requiredChecks, source.sourceCommit, p.maxAgeSeconds, now);
  return checks.filter(c => p.requiredChecks.some(e => e.name === c.name && e.appId === c.app?.id))
    .map(c => ({name: c.name, appId: c.app.id, commit: c.head_sha, conclusion: c.conclusion, url: c.html_url, completedAt: c.completed_at}));
}

export async function authorizePR(api, repo, branch, commit, p, now = Date.now()) {
  const candidates = (await api.list(`repos/${repo}/commits/${commit}/pulls`)).filter(pr => pr.merged_at && pr.merge_commit_sha === commit);
  requireThat(candidates.length === 1, 'commit must resolve to exactly one merged PR');
  const pr = await api.request(`repos/${repo}/pulls/${candidates[0].number}`);
  requireThat(pr.base.repo.full_name === repo && pr.head.repo?.full_name === repo && pr.base.ref === branch && pr.merge_commit_sha === commit, 'fork or wrong base/merge commit');
  const diagnosis = await doctor(api, repo, branch, p);
  requireThat(diagnosis.status === 'passed', `${diagnosis.status}: ${diagnosis.findings.join(' ')}`);
  const reviews = await api.list(`repos/${repo}/pulls/${pr.number}/reviews`);
  const writable = new Set();
  const humans = [pr.merged_by, ...reviews.map(r => r.user)].filter(u => u?.type === 'User');
  for (const login of new Set(humans.map(u => u.login))) {
    const permission = await api.request(`repos/${repo}/collaborators/${encodeURIComponent(login)}/permission`);
    if (['admin','maintain','write'].includes(permission.permission)) writable.add(login);
  }
  validateReviews(pr, reviews, writable, diagnosis.reviewCount);
  const checks = await api.list(`repos/${repo}/commits/${pr.head.sha}/check-runs?filter=latest`, 'check_runs');
  validateChecks(checks, p.prChecks, pr.head.sha, p.maxAgeSeconds, now);
  return pr;
}
