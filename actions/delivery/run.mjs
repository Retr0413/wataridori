import {readFileSync, writeFileSync, mkdirSync, appendFileSync} from 'node:fs';
import {dirname, resolve} from 'node:path';
import {execFileSync} from 'node:child_process';
import {pathToFileURL} from 'node:url';
import {configPath, sha, hash, fingerprint, requireThat, planPath, policy, validatePlan, releaseMarkdown} from './protocol.mjs';
import {GitHub, doctor, quality, authorizePR} from './github.mjs';

const git = (...args) => execFileSync('git', args, {encoding:'utf8', maxBuffer: 8*1024*1024}).trimEnd();
const show = (ref, path) => execFileSync('git', ['show', `${ref}:${path}`], {encoding:'utf8', maxBuffer: 8*1024*1024});
const read = path => readFileSync(path, 'utf8');
const json = path => JSON.parse(read(path));
const save = (path, value) => { mkdirSync(dirname(path), {recursive: true}); writeFileSync(path, JSON.stringify(value, null, 2)+'\n', {mode:0o600}); };
const output = (key, value) => { if (process.env.GITHUB_OUTPUT) appendFileSync(process.env.GITHUB_OUTPUT, `${key}=${value}\n`); };
const cli = args => execFileSync('wataridori',args,{encoding:'utf8'});
function snapshot(env, service) {
  return JSON.parse(execFileSync('wataridori', ['delivery','snapshot','--env',env,'--service',service,'--json'], {encoding:'utf8'}));
}
function select(env, service, ref, allowExpiredObservation = false) {
  const path = planPath(env, service);
  const plan = ref ? JSON.parse(show(ref, path)) : json(path);
  const config = ref ? JSON.parse(show(ref, configPath)) : json(configPath);
  const p = policy(config, env);
  validatePlan(plan, p, env, service, Date.now(), {allowExpiredObservation});
  requireThat(hash(ref ? show(ref, configPath) : read(configPath)) === plan.policyHash, 'delivery policy changed');
  requireThat(hash(ref ? show(ref, 'wataridori.yaml') : read('wataridori.yaml')) === plan.configHash, 'environment configuration changed');
  requireThat(hash(ref ? show(ref, plan.serviceFile) : read(plan.serviceFile)) === plan.afterHash, 'manifest bytes differ from approved plan');
  return {plan,p,path};
}
function freshness(branch, commit, plan, path) {
  git('fetch','origin', `+refs/heads/${branch}:refs/remotes/origin/${branch}`);
  const latest = git('rev-parse', `refs/remotes/origin/${branch}`);
  git('merge-base','--is-ancestor',commit,latest);
  for (const file of [path, plan.serviceFile, configPath, 'wataridori.yaml']) {
    requireThat(show(latest,file) === show(commit,file), 'superseded: reviewed plan or configuration is no longer current');
  }
}

export async function prepare(api, repo, env, service, snapshotter = snapshot, executeCLI = cli) {
  const config = json(configPath), p = policy(config,env);
  const target = snapshotter(env,service);
  requireThat(target.policy === 'manual' && target.applyMode === 'image-only', 'merge-approved requires manual policy and image-only apply');
  const base = process.env.BASE_SHA;
  requireThat(sha.test(base), 'BASE_SHA is required');
  const evidence = json(process.env.EVIDENCE_FILE);
  requireThat(evidence.eligible && !evidence.noop && evidence.to === env && evidence.from === target.promoteFrom && evidence.items.length === 1, 'one eligible source service is required');
  const item = evidence.items[0];
  requireThat(item.service === service && item.eligible && item.desiredImage === target.image && item.actualImage === item.desiredImage, 'evidence does not match promoted service');
  const before = show(base, target.file), after = read(target.file);
  requireThat(before.split(item.targetImage).length === 2 && before.replace(item.targetImage,target.image) === after, 'release must change only the selected image');
  const checks = await quality(api,p,evidence.provenance);
  const verifyArgs = ['delivery','verify','--env',evidence.from,'--service',service,'--image',target.image,'--json'];
  for (const path of p.devHTTPPaths) verifyArgs.push('--http-path',path);
  const liveDev = JSON.parse(executeCLI(verifyArgs));
  requireThat(liveDev.verified, 'live Dev verification failed');
  const status = JSON.parse(executeCLI(['status','--env',env,'--json']));
  const production = status.services.find(s => s.service === service);
  const plan = {version:1, kind:'promotion', state:'proposal_ready', environment:env, service, fromEnvironment:evidence.from,
    image:target.image, oldImage:item.targetImage, applyMode:target.applyMode, serviceFile:target.file,
    beforeHash:hash(before), afterHash:hash(after), configHash:hash(read('wataridori.yaml')), policyHash:hash(read(configPath)),
    createdAt:evidence.createdAt, source:evidence.provenance, evidence:{dev:liveDev, quality:checks, production}, changes:[]};
  // Match a prior plan to the observed production image, never to desired alone.
  // Without a match we explicitly leave the source comparison unavailable.
  try {
    const previous = JSON.parse(show(base,planPath(env,service)));
    requireThat(previous.fingerprint === fingerprint(previous) && production?.actualImage === previous.image && previous.source.sourceRepository === p.sourceRepository, 'previous source is not the observed production image');
    const comparison = await api.request(`repos/${p.sourceRepository}/compare/${previous.source.sourceCommit}...${plan.source.sourceCommit}`);
    requireThat(['ahead','identical'].includes(comparison.status) && comparison.total_commits <= 250 && comparison.commits.length === comparison.total_commits, 'source history diverged or comparison is incomplete');
    plan.attention = (comparison.files ?? []).filter(f => /migration|schema|auth|terraform/i.test(f.filename)).map(f => `Review deployment/rollback compatibility: ${f.filename}`);
    for (const c of comparison.commits) {
      const prs = await api.list(`repos/${p.sourceRepository}/commits/${c.sha}/pulls`);
      plan.changes.push({sha:c.sha,message:c.commit.message.split('\n')[0],url:c.html_url,prs:prs.filter(pr => pr.merged_at && pr.base.repo.full_name === p.sourceRepository).map(pr => ({number:pr.number,url:pr.html_url,title:pr.title}))});
    }
    plan.comparison = 'complete';
  } catch { plan.changes = []; plan.comparison = 'unavailable: cannot prove complete changes from observed production'; }
  plan.fingerprint = fingerprint(plan);
  validatePlan(plan,p,env,service);
  save(planPath(env,service),plan);
  output('plan_path',planPath(env,service));
  if (process.env.SUMMARY_FILE) writeFileSync(process.env.SUMMARY_FILE,releaseMarkdown(plan));
}

export async function admit(api, repo, env, service, snapshotter = snapshot) {
  requireThat(process.env.GITHUB_EVENT_NAME === 'push', 'merge-approved requires a default-branch push');
  const commit = process.env.GITHUB_SHA;
  requireThat(sha.test(commit) && git('rev-parse','HEAD') === commit, 'checkout must equal the triggering immutable commit');
  const metadata = await api.request(`repos/${repo}`), branch = metadata.default_branch;
  requireThat(process.env.GITHUB_REF === `refs/heads/${branch}`, 'not the default branch');
  const {plan,p,path} = select(env,service,commit);
  const associated = (await api.list(`repos/${repo}/commits/${commit}/pulls`)).filter(pr => pr.merged_at && pr.merge_commit_sha === commit && pr.base.repo.full_name === repo && pr.head.repo?.full_name === repo && pr.base.ref === branch);
  if (associated.length === 1) save(process.env.ADMISSION_FILE,{state:'merged_waiting_apply',environment:env,service,commit,branch,pr:associated[0].number,head:associated[0].head.sha,fingerprint:plan.fingerprint,image:plan.image,planPath:path});
  const target = snapshotter(env,service);
  requireThat(target.policy === 'manual' && target.applyMode === plan.applyMode && target.image === plan.image && target.file === plan.serviceFile && target.promoteFrom === plan.fromEnvironment, 'manifest metadata mismatch');
  freshness(branch,commit,plan,path);
  const pr = await authorizePR(api,repo,branch,commit,p);
  try { git('cat-file','-e',`${pr.head.sha}^{commit}`); }
  catch {
    // Squash/rebase merges do not retain the reviewed head in default-branch
    // ancestry. Fetch GitHub's PR ref, never a moving user branch.
    git('fetch','origin',`refs/pull/${pr.number}/head`);
    requireThat(git('rev-parse','FETCH_HEAD') === pr.head.sha,'reviewed PR head changed during fetch');
  }
  const files = await api.list(`repos/${repo}/pulls/${pr.number}/files`);
  requireThat(files.length === 2 && files.every(f => [plan.serviceFile,path].includes(f.filename) && ['added','modified'].includes(f.status)), 'PR must change only one manifest and its release plan');
  const changed = git('diff','--name-only',`${commit}^1`,commit).split('\n');
  requireThat(changed.length === 2 && changed.every(f => [path,plan.serviceFile].includes(f)), 'merge includes unrelated changes');
  for (const ref of [pr.head.sha, commit]) {
    requireThat(show(ref,path) === show(commit,path) && show(ref,plan.serviceFile) === show(commit,plan.serviceFile), 'merge differs from reviewed head');
  }
  const before = show(`${commit}^1`,plan.serviceFile), after = show(commit,plan.serviceFile);
  requireThat(hash(before) === plan.beforeHash && before.split(plan.oldImage).length === 2 && before.replace(plan.oldImage,plan.image) === after, 'only the approved image may change');
  requireThat(show(`${commit}^1`,configPath) === show(commit,configPath) && show(`${commit}^1`,'wataridori.yaml') === show(commit,'wataridori.yaml'), 'configuration changed in release');
  const run = await api.request(`repos/${repo}/actions/runs/${process.env.GITHUB_RUN_ID}`);
  const pin = process.env.WATARIDORI_REF;
  requireThat(sha.test(pin) && run.referenced_workflows?.some(w => w.path === `Retr0413/wataridori/.github/workflows/reusable-prod-merge.yml@${pin}` && w.sha === pin), 'reusable workflow and CLI must use the same full SHA');
  if (plan.kind === 'promotion') await quality(api,p,plan.source);
  else await validateRollback(api,repo,plan);
  const result = {state:'merged_waiting_apply', environment:env,service,commit,branch,pr:pr.number,head:pr.head.sha,fingerprint:plan.fingerprint,image:plan.image,planPath:path};
  save(process.env.ADMISSION_FILE,result);
  output('pr',pr.number); output('image',plan.image); output('kind',plan.kind);
  return result;
}

export async function rollbackPrepare(api,repo,env,service, snapshotter = snapshot, executeCLI = cli) {
  const p = policy(json(configPath),env), target = snapshotter(env,service);
  requireThat(target.policy === 'manual' && target.applyMode === 'image-only','rollback requires manual image-only service');
  const commit = process.env.ROLLBACK_COMMIT, pr = Number(process.env.ROLLBACK_PR);
  requireThat(sha.test(commit) && Number.isSafeInteger(pr) && pr > 0,'ROLLBACK_COMMIT and ROLLBACK_PR identify the verified release');
  const old = JSON.parse(show(commit,planPath(env,service)));
  const before = read(target.file);
  requireThat(old.environment === env && old.service === service && old.image !== target.image && before.split(target.image).length === 2,'invalid rollback target');
  const plan = {version:1,kind:'rollback',state:'proposal_ready',environment:env,service,fromEnvironment:target.promoteFrom,
    image:old.image,oldImage:target.image,applyMode:target.applyMode,serviceFile:target.file,
    beforeHash:hash(before),afterHash:hash(before.replace(target.image,old.image)),
    configHash:hash(read('wataridori.yaml')),policyHash:hash(read(configPath)),createdAt:new Date().toISOString(),
    source:old.source,rollbackOf:{commit,pr},evidence:{previousPlan:old.fingerprint},changes:[]};
  plan.fingerprint = fingerprint(plan); validatePlan(plan,p,env,service);
  await validateRollback(api,repo,plan);
  executeCLI(['manifest','set-image','--env',env,'--service',service,'--image',plan.image,'--require-policy','manual']);
  requireThat(hash(read(target.file)) === plan.afterHash,'rollback manifest hash mismatch');
  save(planPath(env,service),plan);
  if (process.env.SUMMARY_FILE) writeFileSync(process.env.SUMMARY_FILE,releaseMarkdown(plan));
  console.log(`Prepared ${planPath(env,service)}; commit only this plan and ${target.file}, then open a human-reviewed PR.`);
}

async function validateRollback(api,repo,plan) {
  const origin = plan.rollbackOf;
  requireThat(origin && sha.test(origin.commit) && Number.isSafeInteger(origin.pr), 'rollback requires a verified release reference');
  const pr = await api.request(`repos/${repo}/pulls/${origin.pr}`);
  requireThat(pr.merged && pr.merge_commit_sha === origin.commit && pr.base.repo.full_name === repo, 'rollback release reference is not a merged PR');
  const old = JSON.parse(show(origin.commit,planPath(plan.environment,plan.service)));
  requireThat(old.fingerprint === fingerprint(old) && old.image === plan.image && old.source.sourceCommit === plan.source.sourceCommit, 'rollback target differs from referenced release');
  const checks = await api.list(`repos/${repo}/commits/${pr.head.sha}/check-runs?filter=latest`, 'check_runs');
  const check = checks.find(c => c.name === `Wataridori delivery: ${plan.environment}/${plan.service}` && c.app?.slug === 'github-actions' && c.external_id === old.fingerprint && c.conclusion === 'success');
  requireThat(check, 'rollback target has no verified delivery check');
  const runID = check.details_url?.match(new RegExp(`^https://github\\.com/${repo.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')}/actions/runs/([0-9]+)$`))?.[1];
  requireThat(runID, 'verified check must link to the production delivery run');
  const run = await api.request(`repos/${repo}/actions/runs/${runID}`);
  requireThat(run.head_sha === origin.commit && run.event === 'push' && run.conclusion === 'success' &&
    run.referenced_workflows?.some(w => sha.test(w.sha) && w.path === `Retr0413/wataridori/.github/workflows/reusable-prod-merge.yml@${w.sha}`), 'rollback verification was not issued by a successful pinned production workflow');
}

export async function report(api,repo,env,service) {
  const admission = json(process.env.ADMISSION_FILE), plan = JSON.parse(show(admission.commit,planPath(env,service)));
  requireThat(plan.fingerprint === admission.fingerprint && plan.fingerprint === fingerprint(plan),'report plan mismatch');
  let state = process.env.DELIVERY_STATE;
  requireThat(['merged_waiting_apply','deploying','verified','failed','blocked','superseded'].includes(state), 'invalid delivery state');
  try { freshness(admission.branch,admission.commit,plan,planPath(env,service)); }
  catch (error) { state = error.message.includes('superseded:') ? 'superseded' : 'blocked'; }
  let observation;
  try { observation = json(process.env.VERIFICATION_FILE); } catch { /* admission or apply failed */ }
  if (state === 'verified') requireThat(observation?.verified && observation.desiredImage === plan.image && observation.environment === env && observation.service === service, 'cannot report success without production verification');
  const runURL = `https://github.com/${repo}/actions/runs/${process.env.GITHUB_RUN_ID}`;
  const marker = `<!-- wataridori-delivery:${env}:${service}:${admission.fingerprint} -->`;
  const result = {state, fingerprint:admission.fingerprint, commit:admission.commit, image:plan.image, observation:observation ?? null, pr:`https://github.com/${repo}/pull/${admission.pr}`, run:runURL, observedAt:new Date().toISOString()};
  const text = `${marker}\n## Wataridori: ${state}\n\n${runURL}\n\nPlan: \`${plan.fingerprint}\`\n\n${state === 'failed' || state === 'blocked' ? 'Inspect the failed step and actual Cloud Run state before retrying. A reviewed rollback plan can restore a previously verified release. Database changes may require separate recovery.\n' : ''}\n\`\`\`json\n${JSON.stringify(result,null,2).replaceAll('<','\\u003c')}\n\`\`\`\n`;
  const comments = await api.list(`repos/${repo}/issues/${admission.pr}/comments`);
  const previous = comments.find(c => c.user?.login === 'github-actions[bot]' && c.body.startsWith(marker));
  const notified = `<!-- notified:${state} -->`;
  const savedText = text + (previous?.body.includes(notified) ? '\n'+notified : '');
  if (previous) await api.request(`repos/${repo}/issues/comments/${previous.id}`,'PATCH',{body:savedText});
  else await api.request(`repos/${repo}/issues/${admission.pr}/comments`,'POST',{body:text});
  const name = `Wataridori delivery: ${env}/${service}`;
  const checks = await api.list(`repos/${repo}/commits/${admission.head}/check-runs?filter=all`,'check_runs');
  const check = checks.find(c => c.name === name && c.external_id === plan.fingerprint && c.app?.slug === 'github-actions');
  const pending = ['merged_waiting_apply','deploying'].includes(state);
  const body = {name,external_id:plan.fingerprint,details_url:runURL,status:pending ? 'in_progress' : 'completed',
    ...(pending ? {} : {conclusion:state === 'verified' ? 'success' : state === 'superseded' ? 'neutral' : 'failure',completed_at:new Date().toISOString()}),
    output:{title:`${state}: ${env}/${service}`,summary:text.slice(0,60000)}};
  if (check?.conclusion === 'success' && state === 'superseded') {
    // A stale retry must not erase the historical verified release used by
    // rollback. Its PR comment and Actions job still report superseded.
  } else if (check) await api.request(`repos/${repo}/check-runs/${check.id}`,'PATCH',body);
  else await api.request(`repos/${repo}/check-runs`,'POST',{...body,head_sha:admission.head});
  if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY,text);
  // Consumer-controlled secret, never PR/config content. Notification errors are
  // reported separately from deployment and never trigger a redeploy.
  const endpoint = process.env.DELIVERY_WEBHOOK;
  if (endpoint && !pending && !previous?.body.includes(notified)) {
    const url = new URL(endpoint); requireThat(url.protocol === 'https:' && !url.username && !url.password, 'webhook must use HTTPS');
    const response = await fetch(url,{method:'POST',headers:{'Content-Type':'application/json','Idempotency-Key':`${plan.fingerprint}:${state}`},body:JSON.stringify(result),redirect:'error',signal:AbortSignal.timeout(10000)});
    requireThat(response.ok,'deployment result saved; webhook delivery failed');
    const updated = await api.list(`repos/${repo}/issues/${admission.pr}/comments`);
    const comment = updated.find(c => c.user?.login === 'github-actions[bot]' && c.body.startsWith(marker));
    if (comment) await api.request(`repos/${repo}/issues/comments/${comment.id}`,'PATCH',{body:text+'\n'+notified});
  }
}

export async function main(command = process.argv[2]) {
  const env = process.env.ENVIRONMENT, service = process.env.SERVICE, repo = process.env.TARGET_REPOSITORY || process.env.GITHUB_REPOSITORY;
  planPath(env,service); requireThat(/^[\w.-]+\/[\w.-]+$/.test(repo),'GITHUB_REPOSITORY is required');
  const api = new GitHub(process.env.GH_TOKEN);
  if (command === 'prepare') return prepare(api,repo,env,service);
  if (command === 'rollback-prepare') return rollbackPrepare(api,repo,env,service);
  if (command === 'admit') return admit(api,repo,env,service);
  if (command === 'report') return report(api,repo,env,service);
  if (command === 'doctor') {
    const p = policy(json(configPath),env), metadata = await api.request(`repos/${repo}`);
    const result = await doctor(api,repo,metadata.default_branch,p);
    console.log(JSON.stringify(result,null,2));
    if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY,`## Delivery doctor\n\n\`\`\`json\n${JSON.stringify(result,null,2)}\n\`\`\`\n`);
    requireThat(result.status === 'passed','delivery doctor did not pass'); return;
  }
  if (command === 'verify-dev' || command === 'verify-prod') {
    // Evidence expiry must block authorization, not hide actual state after an
    // update that ran past the deadline. Only read-only Prod observation skips TTL.
    const {plan,p} = select(env,service,undefined,command === 'verify-prod');
    if (command === 'verify-dev' && plan.kind === 'rollback') return;
    const dev = command === 'verify-dev', target = dev ? plan.fromEnvironment : env;
    const args = ['delivery','verify','--env',target,'--service',service,'--image',plan.image,'--json'];
    for (const path of dev ? p.devHTTPPaths : p.prodHTTPPaths) args.push('--http-path',path);
    try {
      const result = execFileSync('wataridori',args,{encoding:'utf8'});
      if (!dev && process.env.VERIFICATION_FILE) writeFileSync(process.env.VERIFICATION_FILE,result);
      console.log(result);
    } catch (error) {
      if (!dev && error.stdout && process.env.VERIFICATION_FILE) writeFileSync(process.env.VERIFICATION_FILE,error.stdout);
      throw new Error(`${dev ? 'Dev' : 'Prod'} verification failed; inspect verification output`);
    }
    return;
  }
  throw new Error('expected prepare, doctor, admit, verify-dev, verify-prod, or report');
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch(error => { console.error(error.message); process.exitCode = 1; });
}
