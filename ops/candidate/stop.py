#!/usr/bin/env python3
"""Stop the isolated candidate before resuming the previous deployment."""

import pathlib
import subprocess

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / "var/artifacts/candidate"
CMD = ["docker", "compose", "--project-name", "mini-inference-candidate", "--env-file", "var/artifacts/candidate/runtime.conf", "-f", "compose.yaml", "-f", "compose.candidate.yaml"]


def main():
    from start import candidate_quiescent, check_old_isolated, dmr_unloaded, unpause_old
    check_old_isolated()
    candidate_quiescent()
    subprocess.run([*CMD, "stop", "-t", "75", "web", "api", "controller"], cwd=ROOT, check=True)
    subprocess.run([*CMD, "stop", "-t", "10", "backup-scheduler", "postgres"], cwd=ROOT, check=True)
    subprocess.run([*CMD, "--profile", "restore", "stop", "-t", "10", "restore-drill", "postgres-restore", "restore-evidence-init"], cwd=ROOT, check=True)
    for name in ("mini-inference-candidate-api-1", "mini-inference-candidate-controller-1"):
        state = subprocess.run(["docker", "inspect", "--format", "{{.State.Running}}", name], cwd=ROOT, text=True, stdout=subprocess.PIPE, check=False)
        if state.returncode == 0 and state.stdout.strip() == "true":
            raise RuntimeError(f"candidate {name} is still running")
    dmr_unloaded()
    (OUT / "guard").unlink(missing_ok=True)
    unpause_old()
    print("candidate stopped; previous deployment resumed")


if __name__ == "__main__":
    main()
