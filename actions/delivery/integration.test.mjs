import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {admit,report,rollbackPrepare} from './run.mjs';
import {hash,fingerprint,configPath,planPath} from './protocol.mjs';

test('real Git admission rejects direct pushes, changed bytes, stale runs and approvals; reporting is idempotent', async () => {
  const cwd=process.cwd(), savedEnv={...process.env}, savedFetch=globalThis.fetch, root=mkdtempSync(join(tmpdir(),'wataridori-admission-'));
  const git=(...args)=>execFileSync('git',args,{encoding:'utf8',stdio:['ignore','pipe','pipe']}).trimEnd();
  const put=(file,content)=>{mkdirSync(join(root,'checkout',file,'..'),{recursive:true});writeFileSync(file,content);};
  const now=Date.now(), source='a'.repeat(40), pin='f'.repeat(40), old='reg/app@sha256:'+'a'.repeat(64), image='reg/app@sha256:'+'b'.repeat(64);
  const p={profile:'merge-approved',sourceRepository:'org/app',sourceWorkflowId:7,maxAgeSeconds:3600,requiredChecks:[{name:'tests',appId:42}],prChecks:[{name:'validate',appId:42}],devHTTPPaths:['/ready'],prodHTTPPaths:['/ready']};
  const config=JSON.stringify({version:1,environments:{prod:p}}), file='envs/prod/api.yaml', path=planPath('prod','api');
  try {
    mkdirSync(join(root,'checkout')); process.chdir(join(root,'checkout'));
    git('init','-b','main');git('config','user.name','test');git('config','user.email','test@example.com');
    execFileSync('git',['init','--bare',join(root,'remote.git')],{stdio:'ignore'});
    git('remote','add','origin',join(root,'remote.git'));
    put(configPath,config);put('wataridori.yaml','version: 1\n');
    const before=`name: api\napplyMode: image-only\nimage: ${old}\n`,after=before.replace(old,image);
    put(file,before);git('add','.');git('commit','-m','initial');
    const plan={version:1,kind:'promotion',environment:'prod',service:'api',fromEnvironment:'dev',image,oldImage:old,applyMode:'image-only',serviceFile:file,beforeHash:hash(before),afterHash:hash(after),configHash:hash('version: 1\n'),policyHash:hash(config),createdAt:new Date(now-1000).toISOString(),source:{sourceRepository:'org/app',sourceCommit:source,workflowRun:'https://github.com/org/app/actions/runs/1'}};
    plan.fingerprint=fingerprint(plan);put(path,JSON.stringify(plan));put(file,after);git('add','.');git('commit','-m','release');git('push','origin','main');
    const commit=git('rev-parse','HEAD');
    // A squash merge's reviewed head exists only at the server PR ref, not in
    // the checkout's default-branch history (the source branch may be deleted).
    const head=git('--git-dir',join(root,'remote.git'),'-c','user.name=test','-c','user.email=test@example.com','commit-tree',git('rev-parse','HEAD^{tree}'),'-p',git('rev-parse','HEAD^'),'-m','reviewed proposal');
    git('--git-dir',join(root,'remote.git'),'update-ref','refs/pull/1/head',head);
    assert.throws(()=>git('cat-file','-e',`${head}^{commit}`));
    Object.assign(process.env,{GITHUB_EVENT_NAME:'push',GITHUB_REPOSITORY:'org/app',GITHUB_REF:'refs/heads/main',GITHUB_SHA:commit,GITHUB_RUN_ID:'2',WATARIDORI_REF:pin,ADMISSION_FILE:join(root,'admission.json'),VERIFICATION_FILE:join(root,'verify.json')});
    delete process.env.DELIVERY_WEBHOOK;delete process.env.GITHUB_STEP_SUMMARY;delete process.env.GITHUB_OUTPUT;
    let pr={number:1,merged:true,merged_at:new Date(now).toISOString(),merge_commit_sha:commit,merged_by:{login:'merger',type:'User'},user:{login:'bot',type:'Bot'},head:{sha:head,repo:{full_name:'org/app'}},base:{ref:'main',repo:{full_name:'org/app'}}};
    let review={id:1,state:'APPROVED',commit_id:head,user:{login:'reviewer',type:'User'},submitted_at:new Date(now-500).toISOString()};
    let files=[{filename:file,status:'modified'},{filename:path,status:'added'}];
    let sourceResult='success'; const comments=[],checks=[];
    const api={
      request:async(route,method='GET',body)=>{
        if(method!=='GET') {
          if(route==='repos/org/app/issues/1/comments') {const c={id:1,user:{login:'github-actions[bot]'},...body};comments.push(c);return c;}
          if(route==='repos/org/app/issues/comments/1') {Object.assign(comments[0],body);return comments[0];}
          if(route==='repos/org/app/check-runs') {const c={id:1,app:{slug:'github-actions'},...body};checks.push(c);return c;}
          if(route==='repos/org/app/check-runs/1') {Object.assign(checks[0],body);return checks[0];}
          throw Error('unexpected write '+route);
        }
        if(route==='repos/org/app') return {default_branch:'main'};
        if(route==='repos/org/app/pulls/1') return pr?.number === 1 ? pr : originalPR;
        if(route==='repos/org/app/pulls/2') return pr;
        if(route.includes('/collaborators/')) return {permission:'write'};
        if(route.endsWith('/protection')) return {enforce_admins:{enabled:true},allow_force_pushes:{enabled:false},allow_deletions:{enabled:false},required_pull_request_reviews:{dismiss_stale_reviews:true,required_approving_review_count:1},required_status_checks:{strict:true,checks:[{context:'validate',app_id:42}]}};
        if(route.endsWith('/actions/runs/2') || route.endsWith('/actions/runs/3')) return {head_sha:commit,event:'push',conclusion:'success',referenced_workflows:[{path:`Retr0413/wataridori/.github/workflows/reusable-prod-merge.yml@${pin}`,sha:pin}]};
        if(route.endsWith('/actions/runs/1')) return {repository:{full_name:'org/app'},head_repository:{full_name:'org/app'},head_sha:source,workflow_id:7,event:'push',status:'completed',conclusion:sourceResult};
        throw Error('unexpected read '+route);
      },
      list:async(route)=>{
        if(route.includes('/rules/branches/')) return [];
        if(route.endsWith('/pulls')) return pr ? [pr] : [];
        if(route.endsWith('/reviews')) return [review];
        if(route.endsWith('/files')) return files;
        if(route.endsWith('/comments')) return comments;
        if(route.includes('check-runs?filter=all')) return checks;
        if(pr?.number === 2 && route.includes(`/commits/${head}/check-runs`)) return checks;
        if(route.includes('/check-runs')) return [...(route.includes(`/commits/${head}/`) ? checks : []),{id:1,name:route.includes(source)?'tests':'validate',app:{id:42},head_sha:route.includes(source)?source:pr.head.sha,status:'completed',conclusion:'success',completed_at:new Date(now-1000).toISOString()}];
        throw Error('unexpected list '+route);
      },
    };
    const snap=()=>({policy:'manual',applyMode:'image-only',image,file,promoteFrom:'dev'});
    assert.equal((await admit(api,'org/app','prod','api',snap)).image,image);
    assert.equal(git('rev-parse',`${head}^{commit}`),head,'admission fetched the immutable reviewed head');
    assert.equal((await admit(api,'org/app','prod','api',snap)).commit,commit,'duplicate admission preserves target');
    process.env.GITHUB_EVENT_NAME='workflow_dispatch';await assert.rejects(admit(api,'org/app','prod','api',snap),/push/);process.env.GITHUB_EVENT_NAME='push';
    const originalPR=pr;pr=null;await assert.rejects(admit(api,'org/app','prod','api',snap),/merged PR/);pr=originalPR;
    const originalReview=review;review={...review,commit_id:source};await assert.rejects(admit(api,'org/app','prod','api',snap),/approval/);review=originalReview;
    files=[...files,{filename:'.github/workflows/evil.yml',status:'added'}];await assert.rejects(admit(api,'org/app','prod','api',snap),/only one manifest/);files=files.slice(0,2);
    sourceResult='failure';await assert.rejects(admit(api,'org/app','prod','api',snap),/trusted/);sourceResult='success';
    process.env.DELIVERY_STATE='merged_waiting_apply';await report(api,'org/app','prod','api');assert.equal(checks[0].status,'in_progress');
    process.env.DELIVERY_STATE='verified';writeFileSync(process.env.VERIFICATION_FILE,JSON.stringify({verified:false}));
    await assert.rejects(report(api,'org/app','prod','api'),/without production verification/);
    writeFileSync(process.env.VERIFICATION_FILE,JSON.stringify({verified:true,desiredImage:image,actualImage:image,environment:'prod',service:'api'}));
    await report(api,'org/app','prod','api');await report(api,'org/app','prod','api');
    assert.equal(comments.length,1);assert.equal(checks.length,1);assert.equal(checks[0].conclusion,'success');
    let notifications=0, webhookOK=false;
    process.env.DELIVERY_WEBHOOK='https://consumer.example/delivery';
    globalThis.fetch=async(url,options)=>{
      assert.equal(url.href,process.env.DELIVERY_WEBHOOK);
      assert.equal(options.headers['Idempotency-Key'],`${plan.fingerprint}:verified`);
      assert.equal(options.redirect,'error');
      notifications++;return {ok:webhookOK};
    };
    await assert.rejects(report(api,'org/app','prod','api'),/webhook delivery failed/);
    assert.equal(checks[0].conclusion,'success','notification failure must not mark deployment failed');
    webhookOK=true;
    await report(api,'org/app','prod','api');
    process.env.DELIVERY_STATE='merged_waiting_apply';await report(api,'org/app','prod','api');
    process.env.DELIVERY_STATE='deploying';await report(api,'org/app','prod','api');
    process.env.DELIVERY_STATE='verified';await report(api,'org/app','prod','api');
    assert.equal(notifications,2,'only failed notifications are retried');
    delete process.env.DELIVERY_WEBHOOK;
    // Unrelated default-branch updates do not invalidate the reviewed release.
    put('README.md','unrelated\n');git('add','.');git('commit','-m','docs');git('push','origin','main');git('checkout',commit);
    await admit(api,'org/app','prod','api',snap);
    // A newer plan makes both stale admission and stale success notification invalid.
    git('checkout','main');put(path,JSON.stringify({...plan,createdAt:new Date().toISOString()}));git('add','.');git('commit','-m','new candidate');git('push','origin','main');git('checkout',commit);
    await assert.rejects(admit(api,'org/app','prod','api',snap),/superseded/);
    await report(api,'org/app','prod','api');assert.equal(checks[0].conclusion,'success','stale retry preserves prior verified release');assert.match(comments[0].body,/superseded/);
    assert.equal(JSON.parse(readFileSync(process.env.ADMISSION_FILE,'utf8')).image,image);
    // Create and admit a human-reviewed recovery PR after a newer release.
    // The old source CI/Dev no longer succeeds; verified Prod history authorizes
    // the recovery target, without weakening PR review or freshness checks.
    git('checkout','main');
    const newer='reg/app@sha256:'+'c'.repeat(64), newerManifest=after.replace(image,newer);
    put(file,newerManifest);git('add','.');git('commit','-m','newer desired image');git('push','origin','main');
    Object.assign(process.env,{ROLLBACK_COMMIT:commit,ROLLBACK_PR:'1'});
    await rollbackPrepare(api,'org/app','prod','api',()=>({...snap(),image:newer}),args=>{
      assert.equal(args[0],'manifest');put(file,after);
    });
    git('add','.');git('commit','-m','reviewed recovery');git('push','origin','main');
    const recovery=git('rev-parse','HEAD');
    pr={...originalPR,number:2,merge_commit_sha:recovery,head:{...originalPR.head,sha:recovery}};
    review={...originalReview,commit_id:recovery};files=[{filename:file,status:'modified'},{filename:path,status:'modified'}];
    Object.assign(process.env,{GITHUB_SHA:recovery,GITHUB_RUN_ID:'3'});sourceResult='failure';
    assert.equal((await admit(api,'org/app','prod','api',snap)).image,image);
    assert.equal(JSON.parse(readFileSync(process.env.ADMISSION_FILE,'utf8')).pr,2);
  } finally {
    process.chdir(cwd);
    for(const key of Object.keys(process.env)) if(!(key in savedEnv)) delete process.env[key];
    Object.assign(process.env,savedEnv);
    globalThis.fetch=savedFetch;
  }
});
