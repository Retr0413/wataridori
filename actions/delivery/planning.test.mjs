import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
import {prepare,rollbackPrepare} from './run.mjs';
import {fingerprint,hash,configPath,planPath,validatePlan} from './protocol.mjs';

test('prepare binds live evidence and rollback requires a verified pinned run without deploying', async () => {
  const cwd=process.cwd(),env={...process.env},root=mkdtempSync(join(tmpdir(),'wataridori-plan-'));
  const git=(...args)=>execFileSync('git',args,{encoding:'utf8',stdio:['ignore','pipe','pipe']}).trimEnd();
  const put=(file,value)=>{mkdirSync(join(root,file,'..'),{recursive:true});writeFileSync(file,value);};
  const get=file=>JSON.parse(readFileSync(file,'utf8'));
  try {
    process.chdir(root);git('init','-b','main');git('config','user.name','test');git('config','user.email','test@example.com');
    const now=Date.now(),source='a'.repeat(40),newSource='b'.repeat(40),pin='f'.repeat(40),file='envs/prod/api.yaml',path=planPath('prod','api');
    const p={profile:'merge-approved',sourceRepository:'org/app',sourceWorkflowId:7,maxAgeSeconds:3600,requiredChecks:[{name:'tests',appId:42}],prChecks:[{name:'validate',appId:42}],devHTTPPaths:['/ready'],prodHTTPPaths:['/ready']};
    const config=JSON.stringify({version:1,environments:{prod:p}});
    const old='reg/app@sha256:'+'a'.repeat(64),image='reg/app@sha256:'+'b'.repeat(64);
    const before=`name: api\napplyMode: image-only\nimage: ${old}\n`,after=before.replace(old,image);
    put(configPath,config);put('wataridori.yaml','version: 1\n');put(file,before);
    const previous={version:1,environment:'prod',service:'api',image:old,source:{sourceRepository:'org/app',sourceCommit:source,workflowRun:'https://github.com/org/app/actions/runs/1'}};
    previous.fingerprint=fingerprint(previous);put(path,JSON.stringify(previous));git('add','.');git('commit','-m','previous verified release');
    const base=git('rev-parse','HEAD');put(file,after);
    const evidence={eligible:true,noop:false,to:'prod',from:'dev',createdAt:new Date(now-1000).toISOString(),provenance:{sourceRepository:'org/app',sourceCommit:newSource,workflowRun:'https://github.com/org/app/actions/runs/2'},items:[{service:'api',eligible:true,desiredImage:image,actualImage:image,targetImage:old}]};
    put('evidence.json',JSON.stringify(evidence));Object.assign(process.env,{BASE_SHA:base,EVIDENCE_FILE:join(root,'evidence.json'),SUMMARY_FILE:join(root,'summary.md'),ROLLBACK_COMMIT:base,ROLLBACK_PR:'10'});delete process.env.GITHUB_OUTPUT;
    let succeeded=true,liveDev=true;const cliCalls=[];
    const api={request:async(route)=>{
      if(route.endsWith('/actions/runs/2'))return {repository:{full_name:'org/app'},head_repository:{full_name:'org/app'},head_sha:newSource,workflow_id:7,event:'push',status:'completed',conclusion:'success'};
      if(route.includes('/compare/'))return {status:'ahead',total_commits:1,commits:[{sha:newSource,commit:{message:'fix API'},html_url:`https://github.com/org/app/commit/${newSource}`}],files:[{filename:'migrations/001.sql'}]};
      if(route.endsWith('/pulls/10'))return {merged:true,merge_commit_sha:base,base:{repo:{full_name:'org/app'}},head:{sha:base}};
      if(route.endsWith('/actions/runs/10'))return {head_sha:base,event:'push',conclusion:succeeded?'success':'failure',referenced_workflows:[{path:`Retr0413/wataridori/.github/workflows/reusable-prod-merge.yml@${pin}`,sha:pin}]};
      throw Error('unexpected '+route);
    },list:async(route)=>{
      if(route.endsWith('/pulls'))return [{number:2,merged_at:new Date(now).toISOString(),base:{repo:{full_name:'org/app'}},html_url:'https://github.com/org/app/pull/2',title:'fix API'}];
      if(route.includes(`/commits/${base}/check-runs`))return [{name:'Wataridori delivery: prod/api',app:{slug:'github-actions'},external_id:previous.fingerprint,conclusion:'success',details_url:'https://github.com/org/app/actions/runs/10'}];
      if(route.includes('/check-runs'))return [{id:1,name:'tests',app:{id:42},head_sha:newSource,status:'completed',conclusion:'success',completed_at:new Date(now-1000).toISOString()}];
      throw Error('unexpected '+route);
    }};
    const snap=()=>({policy:'manual',applyMode:'image-only',image,file,promoteFrom:'dev'});
    const execute=args=>{cliCalls.push(args);if(args[0]==='status')return JSON.stringify({services:[{service:'api',actualImage:old,ready:true}]});return JSON.stringify({verified:liveDev,desiredImage:image});};
    await prepare(api,'org/app','prod','api',snap,execute);
    const plan=get(path);validatePlan(plan,p,'prod','api');assert.equal(plan.beforeHash,hash(before));assert.equal(plan.afterHash,hash(after));assert.equal(plan.changes[0].prs[0].number,2);assert.match(plan.attention[0],/migrations/);assert.match(readFileSync(process.env.SUMMARY_FILE,'utf8'),/migrations\/001.sql/);assert.ok(cliCalls.some(args=>args.includes('/ready')));
    liveDev=false;await assert.rejects(prepare(api,'org/app','prod','api',snap,execute),/Dev/);liveDev=true;
    put(file,after+'port: 9000\n');await assert.rejects(prepare(api,'org/app','prod','api',snap,execute),/only the selected image/);put(file,after);
    let mutations=0;
    const update=args=>{mutations++;assert.deepEqual(args,['manifest','set-image','--env','prod','--service','api','--image',old,'--require-policy','manual']);put(file,before);};
    succeeded=false;await assert.rejects(rollbackPrepare(api,'org/app','prod','api',snap,update),/successful pinned/);assert.equal(mutations,0);
    succeeded=true;await rollbackPrepare(api,'org/app','prod','api',snap,update);
    const rollback=get(path);validatePlan(rollback,p,'prod','api');assert.equal(rollback.kind,'rollback');assert.equal(rollback.image,old);assert.equal(rollback.oldImage,image);assert.equal(rollback.rollbackOf.commit,base);assert.equal(mutations,1);
    assert.ok(!cliCalls.some(args=>args[0]==='apply'));
  }finally{process.chdir(cwd);for(const k of Object.keys(process.env))if(!(k in env))delete process.env[k];Object.assign(process.env,env);}
});
