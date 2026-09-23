#!/usr/bin/env python3
import os,pathlib,re,subprocess
root=pathlib.Path(__file__).resolve().parents[2]
candidate=os.environ.get('MINI_CANDIDATE_RESTORE')=='1'
selection=root/'var'/'artifacts'/('candidate/restore-selection' if candidate else 'restore-selection')
if selection.is_symlink() or not selection.is_file(): raise SystemExit('restore selection is missing or unsafe')
name=selection.read_text().strip()
if not re.fullmatch(r'mini-inference-\d{8}T\d{6}Z-7\.dump',name): raise SystemExit('invalid restore selection')
env=os.environ.copy(); env['RESTORE_ARCHIVE']=name
project='mini-inference-candidate' if candidate else 'mini-inference'
config='var/artifacts/candidate/runtime.conf' if candidate else 'config/runtime.mac.conf'
cmd=['docker','compose','--project-name',project,'--env-file',config,'-f','compose.yaml']
if candidate: cmd += ['-f','compose.candidate.yaml']
if candidate:
    query=[*cmd,'--profile','ops','run','--rm','--no-deps','--entrypoint','/bin/sh','migrate','-ec','dsn=$(cat /run/secrets/mini_migrator_dsn); exec psql --dbname="$dsn" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "SELECT public_model_id FROM public_model_identity WHERE singleton"']
    current=subprocess.run(query,cwd=root,env=env,text=True,capture_output=True,check=True).stdout.strip()
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}',current): raise SystemExit('invalid current candidate public model name')
    expected=root/'var'/'artifacts'/'candidate'/'expected-public-model-id'
    if expected.is_symlink() or not expected.is_file(): raise SystemExit('candidate expected-name file is unsafe')
    expected.write_text(current+'\n')
cmd += ['--profile','restore','up','--abort-on-container-exit','--exit-code-from','restore-drill','restore-drill']
raise SystemExit(subprocess.run(cmd,cwd=root,env=env,check=False).returncode)
