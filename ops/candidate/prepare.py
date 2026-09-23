#!/usr/bin/env python3
"""Prepare and inspect the isolated Compose candidate without starting services."""

import hashlib
import json
import os
import pathlib
import subprocess
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / "var/artifacts/candidate"
PROJECT = "mini-inference-candidate"
ZERO = "0" * 64


def fail(message):
    raise SystemExit(message)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def update_env(path, **updates):
    values = dict(line.split("=", 1) for line in path.read_text().splitlines() if line and not line.startswith("#"))
    values.update(updates)
    path.write_text("\n".join(f"{key}={value}" for key, value in values.items()) + "\n")


def compose(env_file, *args):
    cmd = ["docker", "compose", "--project-name", PROJECT, "--env-file", str(env_file),
           "-f", "compose.yaml", "-f", "compose.candidate.yaml", "--profile", "ops", "--profile", "restore", "config", *args]
    return subprocess.check_output(cmd, cwd=ROOT, text=True)


def main():
    source = json.loads((ROOT / "config/compatibility-manifest.json").read_text())
    if source["release"]["status"] != "unresolved" or source["database"]["migration_version"] != 7:
        fail("production candidate source must remain unresolved at schema 7")
    runtime = json.loads((ROOT / "var/artifacts/compatible-runtime/state/runtime.json").read_text())
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        with opener.open("http://127.0.0.1:12435/mini-inference/v1/identity", timeout=3) as response:
            live = json.load(response)
    except (urllib.error.URLError, TimeoutError):
        fail("approved loopback DMR identity is unavailable")
    for key in ("dmr_sha256", "llama_sha256"):
        if live.get(key) != runtime.get(key):
            fail("live DMR identity differs from the pinned runtime")
    if live.get("model_digest") != runtime["model_digest"].removeprefix("sha256:") or live["model_digest"] != source["model"]["oci_digest"].removeprefix("sha256:"):
        fail("live model identity differs from the manifest")

    builds = {"model": live["model_digest"]}
    for name in ("api", "web", "controller", "jobs"):
        item = source["images"][name]
        expected = item["reference"]
        if expected != item["digest"] or not expected.startswith("sha256:"):
            fail(f"unresolved {name} image identity")
        observed = subprocess.check_output(["docker", "image", "inspect", "--format", "{{.Id}} {{.Os}}/{{.Architecture}}", expected], text=True).strip()
        if observed != expected + " linux/arm64":
            fail(f"local {name} image identity mismatch")
        builds[name] = expected.removeprefix("sha256:")
    builds["dmr"] = live["dmr_sha256"]
    builds["llama"] = live["llama_sha256"]

    for path in (ROOT / "var", ROOT / "var/artifacts", OUT, OUT / "backups"):
        if path.is_symlink():
            fail("candidate path may not be a symbolic link")
    OUT.mkdir(parents=True, exist_ok=True)
    backups = OUT / "backups"
    backups.mkdir(mode=0o700, exist_ok=True)
    os.chmod(backups, 0o700)
    manifest_path = OUT / "compatibility-manifest.json"
    env_path = OUT / "runtime.conf"
    expected_name_file = OUT / "expected-public-model-id"
    for path in (manifest_path, env_path, OUT / "compose.resolved.yaml", OUT / "guard", OUT / "guard.partial", OUT / "recovery.json", expected_name_file):
        if path.is_symlink():
            fail("candidate artifact may not be a symbolic link")
    if (OUT / "guard").exists():
        fail("candidate is active; stop it before preparing another identity")
    manifest = source
    manifest["release"]["status"] = "candidate"
    manifest["release"]["target_environment"] = "apple-silicon-mac-local-candidate"
    manifest["privacy_gate"]["status"] = "unresolved"
    manifest["privacy_gate"]["passed"] = False
    manifest["privacy_gate"]["evidence_id"] = None
    manifest["privacy_gate"]["exact_build_set"] = ";".join(f"{key}:{value}" for key, value in builds.items())
    manifest["evidence"]["privacy"] = None
    manifest["resolved_compose_config_sha256"] = ZERO
    write_json(manifest_path, manifest)
    expected_name_file.write_text("")
    expected_name_file.chmod(0o644)
    env_path.write_bytes((ROOT / "config/runtime.mac.conf").read_bytes())
    update_env(env_path, COMPATIBILITY_MANIFEST_SHA256=digest(manifest_path), COMPOSE_CONFIG_SHA256=ZERO)

    raw = compose(env_path)
    canonical = "\n".join(line for line in raw.splitlines() if not any(
        marker in line for marker in ("RESOLVED_COMPOSE_CONFIG_SHA256:", "COMPOSE_CONFIG_SHA256:", "COMPATIBILITY_MANIFEST_SHA256:"))) + "\n"
    config_hash = hashlib.sha256(canonical.encode()).hexdigest()
    manifest["resolved_compose_config_sha256"] = config_hash
    write_json(manifest_path, manifest)
    update_env(env_path, COMPATIBILITY_MANIFEST_SHA256=digest(manifest_path), COMPOSE_CONFIG_SHA256=config_hash)
    rendered = compose(env_path)
    (OUT / "compose.resolved.yaml").write_text(rendered)
    config = json.loads(compose(env_path, "--format", "json"))
    inspect_topology(config, source, backups, manifest_path)
    print("candidate identity and isolated Compose topology verified; no services started")


def inspect_topology(config, manifest, backups, manifest_path):
    if config["name"] != PROJECT:
        fail("candidate project name mismatch")
    services = config["services"]
    if set(services) != {"api", "backup-prune", "backup-scheduler", "controller", "lifecycle-proof", "migrate", "postgres", "postgres-restore", "restore-drill", "restore-evidence-init", "restore-proof-grant", "restore-proof-recorder", "restore-proof-revoke", "web"}:
        fail("unexpected candidate service topology")
    allowed_images = {item["reference"] for item in manifest["images"].values()}
    for name, service in services.items():
        ports = service.get("ports", [])
        expected = {"api": (8888, "18888"), "web": (8080, "18080")}.get(name)
        if expected:
            if len(ports) != 1 or ports[0].get("host_ip") != "127.0.0.1" or (ports[0].get("target"), str(ports[0].get("published"))) != expected:
                fail(f"unsafe candidate {name} port")
        elif ports:
            fail(f"unexpected published port on {name}")
        if service.get("image") not in allowed_images:
            fail(f"candidate {name} image mismatch")
        for volume in service.get("volumes", []):
            source = str(volume.get("source", ""))
            if source in (str(ROOT / "var/backups/postgres"), "mini-inference_postgres-data") or "docker.sock" in source:
                fail(f"unsafe candidate {name} mount")
    if services["api"].get("environment", {}).get("VERIFICATION_MODE") != "candidate":
        fail("candidate mode missing")
    if services["api"]["environment"].get("AI_MODEL_NAME") != manifest["model"]["oci_reference"] or services["controller"]["environment"].get("MODEL_ARTIFACT_REF") != manifest["model"]["oci_reference"]:
        fail("candidate model reference mismatch")
    if services["api"]["environment"].get("AI_MODEL_URL") != "http://model-runner.docker.internal:12435" or services["controller"]["environment"].get("MODEL_RUNNER_HOST") != "http://model-runner.docker.internal:12435":
        fail("candidate DMR route mismatch")
    if services["api"].get("deploy", {}).get("replicas", 1) != 1:
        fail("candidate API replica count mismatch")
    controller_env = services["controller"].get("environment", {})
    if controller_env.get("VERIFICATION_MODE") != "candidate" or controller_env.get("CANDIDATE_GUARD_FILE") != "/run/candidate.guard" or controller_env.get("CANDIDATE_MANIFEST_SHA256") != hashlib.sha256(manifest_path.read_bytes()).hexdigest():
        fail("candidate controller guard missing")
    guard_mounts = [v for v in services["controller"].get("volumes", []) if v.get("target") == "/run/candidate.guard"]
    if len(guard_mounts) != 1 or guard_mounts[0].get("source") != str(OUT / "guard") or not guard_mounts[0].get("read_only"):
        fail("candidate guard mount mismatch")
    for name in ("backup-scheduler", "backup-prune", "restore-drill"):
        if not any(v.get("source") == str(backups) and v.get("target") == "/backups" for v in services[name].get("volumes", [])):
            fail(f"candidate {name} backup isolation missing")
    if services["restore-drill"].get("environment", {}).get("EXPECTED_PUBLIC_MODEL_ID_FILE") != "/run/candidate/expected-public-model-id" or not any(v.get("source") == str(OUT / "expected-public-model-id") and v.get("target") == "/run/candidate/expected-public-model-id" and v.get("read_only") for v in services["restore-drill"].get("volumes", [])):
        fail("candidate restore name comparison missing")
    if config["configs"]["compatibility_manifest"]["file"] != str(manifest_path):
        fail("candidate manifest mount mismatch")
    if config["volumes"]["postgres-data"]["name"] == "mini-inference_postgres-data":
        fail("candidate database volume overlaps production")


if __name__ == "__main__":
    main()
