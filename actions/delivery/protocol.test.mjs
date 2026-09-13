import test from 'node:test';
import assert from 'node:assert/strict';
import {fingerprint, hash, policy, validatePlan, validateChecks, validateReviews, protectionFindings, planPath} from './protocol.mjs';
import {GitHub, doctor, quality} from './github.mjs';

export const now = Date.now();
export const sourceSHA = 'a'.repeat(40);
export const checkSpec = {name:'tests',appId:42};
export const p = {profile:'merge-approved',sourceRepository:'org/app',sourceWorkflowId:7,maxAgeSeconds:3600,requiredChecks:[checkSpec],prChecks:[checkSpec],devHTTPPaths:['/ready'],prodHTTPPaths:['/ready']};
export const passing = {id:1,name:'tests',app:{id:42},head_sha:sourceSHA,status:'completed',conclusion:'success',completed_at:new Date(now-1000).toISOString()};
export const protection = {enforce_admins:{enabled:true},allow_force_pushes:{enabled:false},allow_deletions:{enabled:false},required_pull_request_reviews:{dismiss_stale_reviews:true,required_approving_review_count:1},required_status_checks:{strict:true,checks:[{context:'tests',app_id:42}]}};
export function fixturePlan() {
  const plan = {version:1,kind:'promotion',environment:'prod',service:'api',fromEnvironment:'dev',image:'reg/app@sha256:'+'b'.repeat(64),oldImage:'reg/app@sha256:'+'a'.repeat(64),applyMode:'image-only',serviceFile:'envs/prod/api.yaml',beforeHash:hash('old'),afterHash:hash('new'),configHash:hash('config'),policyHash:hash('policy'),createdAt:new Date(now-1000).toISOString(),source:{sourceRepository:'org/app',sourceCommit:sourceSHA,workflowRun:'https://github.com/org/app/actions/runs/1'}};
  plan.fingerprint = fingerprint(plan); return plan;
}
test('fingerprint binds every plan field and ignores object property order', () => {
  const plan = fixturePlan();
  assert.equal(fingerprint(plan),fingerprint(Object.fromEntries(Object.entries(plan).reverse())));
  for (const key of ['image','applyMode','serviceFile','createdAt','beforeHash','configHash','policyHash']) {
    const changed = {...plan,[key]:'changed'};
    assert.notEqual(fingerprint(changed),plan.fingerprint);
    assert.throws(() => validatePlan(changed,p,'prod','api',now));
  }
  validatePlan(plan,p,'prod','api',now);
});
test('plan rejects replay, future clock, mutable references and traversals even with recomputed fingerprint', () => {
  for (const update of [{createdAt:new Date(now-3601000).toISOString()},{createdAt:new Date(now+1000).toISOString()},{image:'reg/app:latest'},{serviceFile:'../secret.yaml'},{serviceFile:'/envs/prod/api.yaml'},{kind:'auto-repair'},{source:{sourceRepository:'evil/app',sourceCommit:sourceSHA,workflowRun:'https://github.com/evil/app/actions/runs/1'}}]) {
    const plan = {...fixturePlan(),...update}; plan.fingerprint=fingerprint(plan);
    assert.throws(() => validatePlan(plan,p,'prod','api',now));
  }
  assert.throws(() => planPath('../prod','api'));
  const expired={...fixturePlan(),createdAt:new Date(now-3601000).toISOString()};expired.fingerprint=fingerprint(expired);
  assert.doesNotThrow(()=>validatePlan(expired,p,'prod','api',now,{allowExpiredObservation:true}));
  assert.throws(()=>validatePlan(expired,p,'prod','api',now),'observation exception must not change admission defaults');
});
test('delivery is explicitly opt-in and requires pinned publishers and bounded health paths', () => {
  assert.equal(policy({version:1,environments:{prod:p}},'prod'),p);
  for (const update of [{profile:'manual'},{requiredChecks:[]},{prChecks:[{name:'tests',appId:-1}]},{maxAgeSeconds:999999},{devHTTPPaths:['//evil.example/path']},{prodHTTPPaths:['https://evil.example/path']}]) assert.throws(() => policy({version:1,environments:{prod:{...p,...update}}},'prod'));
});
test('quality fails closed on every non-success, missing, wrong commit, publisher, expired or latest failed rerun', () => {
  validateChecks([passing],[checkSpec],sourceSHA,3600,now);
  for (const update of [{conclusion:'failure'},{conclusion:'cancelled'},{conclusion:'skipped'},{conclusion:'neutral'},{conclusion:null,status:'queued'},{head_sha:'b'.repeat(40)},{app:{id:999}},{completed_at:new Date(now-3601000).toISOString()},{completed_at:null}]) assert.throws(() => validateChecks([{...passing,...update}],[checkSpec],sourceSHA,3600,now));
  assert.throws(() => validateChecks([],[checkSpec],sourceSHA,3600,now));
  assert.throws(() => validateChecks([passing,{...passing,id:2,conclusion:'failure'}],[checkSpec],sourceSHA,3600,now));
});
test('only latest human review before merge on exact head counts; comments preserve approval', () => {
  const pr={merged:true,merged_at:new Date(now).toISOString(),merged_by:{login:'merger',type:'User'},head:{sha:sourceSHA},user:{login:'author'}};
  const review={id:1,state:'APPROVED',commit_id:sourceSHA,user:{login:'reviewer',type:'User'},submitted_at:new Date(now-1000).toISOString()};
  const writable=new Set(['merger','reviewer']);
  validateReviews(pr,[review,{...review,id:2,state:'COMMENTED'}],writable,1);
  for (const change of [{state:'DISMISSED'},{state:'CHANGES_REQUESTED'},{commit_id:'b'.repeat(40)},{user:{login:'reviewer',type:'Bot'}},{submitted_at:new Date(now+1000).toISOString()}]) assert.throws(() => validateReviews(pr,[{...review,...change}],writable,1));
  assert.throws(() => validateReviews({...pr,merged:false},[review],writable,1));
  assert.throws(() => validateReviews({...pr,user:{login:'reviewer'}},[review],writable,1));
  assert.throws(() => validateReviews(pr,[review],new Set(['merger']),1));
  assert.throws(() => validateReviews(pr,[review,{...review,id:2,state:'DISMISSED'}],writable,1));
});
test('doctor requires effective non-bypass protection and exact required check publisher', async () => {
  assert.equal(protectionFindings(protection,[],[],p).status,'passed');
  for (const update of [{enforce_admins:{enabled:false}},{allow_force_pushes:{enabled:true}},{allow_deletions:{enabled:true}},{required_status_checks:{strict:false}},{required_pull_request_reviews:{...protection.required_pull_request_reviews,bypass_pull_request_allowances:{apps:[{id:1}]}}}]) assert.equal(protectionFindings({...protection,...update},[],[],p).status,'blocked');
  const rules=[{type:'pull_request',ruleset_id:1,parameters:{dismiss_stale_reviews_on_push:true,required_approving_review_count:1}},{type:'required_status_checks',ruleset_id:1,parameters:{strict_required_status_checks_policy:true,required_status_checks:[{context:'tests',integration_id:42}]}},{type:'non_fast_forward',ruleset_id:1},{type:'deletion',ruleset_id:1}];
  assert.equal(protectionFindings(null,rules,[{id:1,enforcement:'active',bypass_actors:[]}],p).status,'passed');
  assert.equal(protectionFindings(null,rules,[{id:1,enforcement:'active',bypass_actors:[{}]}],p).status,'blocked');
  assert.equal(protectionFindings(null,rules,[{id:1,enforcement:'active'}],p).status,'blocked','missing bypass visibility is not proof of no bypass');
  const result=await doctor({request:async()=>{throw Error('HTTP 403');}},'org/app','main',p);
  assert.equal(result.status,'unknown');
});
test('API follows all pages and never forwards authorization across redirects', async () => {
  let calls=0;
  const api=new GitHub('token',async(url,options)=>{calls++; assert.equal(options.redirect,'error'); return {ok:true,status:200,json:async()=>new URL(url).searchParams.get('page') === '1'?Array(100).fill({id:1}):[{id:2}]};});
  assert.equal((await api.list('repos/org/app/pulls')).length,101); assert.equal(calls,2);
});
test('source workflow must be trusted and tied to artifact source SHA', async () => {
  const run={repository:{full_name:'org/app'},head_repository:{full_name:'org/app'},head_sha:sourceSHA,workflow_id:7,event:'push',status:'completed',conclusion:'success'};
  const source=fixturePlan().source;
  for (const update of [{head_sha:'b'.repeat(40)},{workflow_id:8},{event:'pull_request_target'},{head_repository:{full_name:'fork/app'}},{conclusion:'failure'}]) {
    await assert.rejects(quality({request:async()=>({...run,...update}),list:async()=>[passing]},p,source,now));
  }
  assert.equal((await quality({request:async()=>run,list:async()=>[passing]},p,source,now)).length,1);
});
