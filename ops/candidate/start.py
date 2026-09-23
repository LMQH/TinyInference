#!/usr/bin/env python3
"""Quiesce the current controller and start the isolated candidate."""

import json
import pathlib
import subprocess
import sys
import time

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / "var/artifacts/candidate"
GUARD = OUT / "guard"
OLD = ("mini-inference-web-1", "mini-inference-api-1", "mini-inference-controller-1")
CANDIDATE = ["docker", "compose", "--project-name", "mini-inference-candidate", "--env-file", "var/artifacts/candidate/runtime.conf", "-f", "compose.yaml", "-f", "compose.candidate.yaml"]


def run(*args, capture=False):
    return subprocess.run(args, cwd=ROOT, check=True, text=True, stdout=subprocess.PIPE if capture else None).stdout


def candidate(*args):
    return run(*CANDIDATE, *args)


def states():
    raw = run("docker", "inspect", "--format", "{{json .}}", *OLD, capture=True)
    items = [json.loads(line) for line in raw.splitlines()]
    return {item["Name"].lstrip("/"): item for item in items}


def snapshot():
    raw = run("docker", "exec", OLD[0], "wget", "-qO-", "http://api:8889/admin/v1/snapshot", capture=True)
    data = json.loads(raw)
    if data["model"]["state"] != "unloaded" or data["queue"]["depth"] != 0 or data["queue"]["active"] is not None:
        raise RuntimeError("existing model or queue is active")


def dmr_unloaded():
    raw = run("curl", "--noproxy", "*", "--fail", "--silent", "--show-error", "--max-time", "4", "http://127.0.0.1:12435/engines/ps", capture=True)
    if json.loads(raw) != []:
        raise RuntimeError("DMR model is still loaded")


def old_database_idle():
    sql = "select (select count(*) from inference_requests where completed_at is null), (select count(*) from model_operations where completed_at is null)"
    raw = run("docker", "exec", "mini-inference-backup-scheduler-1", "sh", "-ec",
              'psql "$(cat "$DATABASE_URL_FILE")" -X -A -t -c "$1"', "sh", sql, capture=True)
    if raw.strip() != "0|0":
        raise RuntimeError("existing requests or model operations are still active")


def expect_old(paused):
    current = states()
    for name in OLD:
        item = current[name]
        if item["State"]["Paused"] != paused or not item["State"]["Running"]:
            raise RuntimeError(f"existing {name} state does not match quiescence")
        if not paused and item["State"].get("Health", {}).get("Status") != "healthy":
            raise RuntimeError(f"existing {name} is not healthy")
    return current


def check_old_isolated():
    current = expect_old(True)
    recorded = json.loads((OUT / "recovery.json").read_text())
    for name in OLD:
        if current[name]["Id"] != recorded[name]["id"] or current[name]["Image"] != recorded[name]["image"]:
            raise RuntimeError(f"existing {name} identity changed")


def pause_old():
    try:
        for name in OLD:
            run("docker", "pause", name)
        expect_old(True)
        old_database_idle()
        dmr_unloaded()
        time.sleep(1)
        dmr_unloaded()
    except BaseException:
        unpause_old()
        raise


def candidate_quiescent():
    deadline = time.monotonic() + 90
    stop_requested = False
    while time.monotonic() < deadline:
        try:
            raw = run("curl", "--noproxy", "*", "--fail", "--silent", "--show-error", "--max-time", "4",
                      "http://127.0.0.1:18080/admin/v1/snapshot", capture=True)
            data = json.loads(raw)
        except (subprocess.CalledProcessError, ValueError):
            dmr_unloaded()
            return
        if data["model"]["state"] == "unloaded" and data["queue"]["active"] is None and data["queue"]["depth"] == 0:
            dmr_unloaded()
            return
        if data["model"]["state"] == "unavailable" and data["queue"]["active"] is None and data["queue"]["depth"] == 0:
            try:
                dmr_unloaded()
                return
            except RuntimeError:
                pass
        if data["model"]["state"] in ("ready", "unavailable") and not stop_requested:
            run("curl", "--noproxy", "*", "--fail", "--silent", "--show-error", "--max-time", "4",
                "-X", "POST", "http://127.0.0.1:18080/admin/v1/model/stop", capture=True)
            stop_requested = True
        time.sleep(2)
    raise RuntimeError("candidate model or queue did not become idle")


def unpause_old():
    for name in reversed(OLD):
        if states()[name]["State"]["Paused"]:
            run("docker", "unpause", name)
    deadline = time.monotonic() + 40
    while time.monotonic() < deadline:
        try:
            expect_old(False)
            snapshot()
            return
        except (RuntimeError, subprocess.CalledProcessError):
            time.sleep(2)
    raise RuntimeError("existing deployment did not recover after unpause")


def main():
    if GUARD.exists() or GUARD.is_symlink():
        raise RuntimeError("candidate guard already exists")
    run(sys.executable, "ops/candidate/prepare.py")
    before = expect_old(False)
    snapshot()
    dmr_unloaded()
    # A short rehearsal proves the running, exact old containers can resume.
    try:
        pause_old()
    finally:
        unpause_old()
    after = expect_old(False)
    for name in OLD:
        if after[name]["Id"] != before[name]["Id"] or after[name]["Image"] != before[name]["Image"]:
            raise RuntimeError("existing container changed during recovery rehearsal")
    (OUT / "recovery.json").write_text(json.dumps({name: {"id": before[name]["Id"], "image": before[name]["Image"]} for name in OLD}, indent=2) + "\n")

    try:
        pause_old()
        manifest_hash = dict(line.split("=", 1) for line in (OUT / "runtime.conf").read_text().splitlines() if "=" in line)["COMPATIBILITY_MANIFEST_SHA256"]
        temporary = OUT / "guard.partial"
        temporary.write_text(manifest_hash + "\n")
        temporary.chmod(0o644)
        temporary.replace(GUARD)
        candidate("up", "-d", "postgres")
        candidate("--profile", "ops", "run", "--rm", "migrate")
        candidate("up", "-d", "postgres", "backup-scheduler", "controller", "api", "web")
        check_old_isolated()
        print("isolated candidate started; existing web/API/controller are paused")
    except BaseException:
        if GUARD.exists():
            candidate_quiescent()
            candidate("stop", "-t", "75", "web", "api", "controller")
            candidate("stop", "-t", "10", "backup-scheduler", "postgres")
            dmr_unloaded()
            GUARD.unlink(missing_ok=True)
        unpause_old()
        raise


if __name__ == "__main__":
    main()
