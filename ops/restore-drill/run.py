#!/usr/bin/env python3
import os,pathlib,re,subprocess
root=pathlib.Path(__file__).resolve().parents[2]
selection=root/'var'/'artifacts'/'restore-selection'
if selection.is_symlink() or not selection.is_file(): raise SystemExit('restore selection is missing or unsafe')
name=selection.read_text().strip()
if not re.fullmatch(r'mini-inference-\d{8}T\d{6}Z-6\.dump',name): raise SystemExit('invalid restore selection')
env=os.environ.copy(); env['RESTORE_ARCHIVE']=name
cmd=['docker','compose','--project-name','mini-inference','--env-file','config/runtime.mac.conf','--profile','restore','up','--abort-on-container-exit','--exit-code-from','restore-drill','restore-drill']
raise SystemExit(subprocess.run(cmd,cwd=root,env=env,check=False).returncode)
