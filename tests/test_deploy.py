"""Offline tests for deploy/deploy.sh (blue-green on the staging box) and
deploy/backup.sh. No stack, no network: deploy.sh runs against fake
`doppler` / `docker` / `curl` / `nginx` / `systemctl` / `hostname` / `sleep`
on PATH, in a temp deploy dir with a temp nginx upstream file.

Run alone: uv run --directory tests pytest test_deploy.py -q
"""

import json
import os
import shutil
import sqlite3
import stat
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parent.parent
DEPLOY = ROOT / "deploy" / "deploy.sh"
BACKUP = ROOT / "deploy" / "backup.sh"
SHA = "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"

FAKES = {
    # doppler run -- <cmd...>: run the command with the fake APP_ENV.
    "doppler": """#!/usr/bin/env bash
echo "doppler $*" >>"$FAKE_LOG"
[[ "$1" == run ]] || exit 2
shift; [[ "$1" == -- ]] && shift
APP_ENV="$FAKE_APP_ENV" exec "$@"
""",
    "docker": """#!/usr/bin/env bash
echo "docker $*" >>"$FAKE_LOG"
exit "${FAKE_DOCKER_RC:-0}"
""",
    # /health on a slot: FAKE_HEALTH=ok|fail. /api/v1/version on a slot:
    # FAKE_SLOT_JSON. https:// (through nginx): FAKE_NGINX_JSON.
    "curl": """#!/usr/bin/env bash
url="${@: -1}"
echo "curl $url" >>"$FAKE_LOG"
case "$url" in
  http://127.0.0.1:*/health) [[ "${FAKE_HEALTH:-ok}" == ok ]] && { echo OK; exit 0; }; exit 7 ;;
  http://127.0.0.1:*/api/v1/version) echo "$FAKE_SLOT_JSON" ;;
  https://*/api/v1/version) echo "$FAKE_NGINX_JSON" ;;
  *) exit 22 ;;
esac
""",
    # nginx -t fails when the upstream file points at FAKE_NGINX_BAD_PORT.
    "nginx": """#!/usr/bin/env bash
echo "nginx $*" >>"$FAKE_LOG"
if [[ -n "${FAKE_NGINX_BAD_PORT:-}" ]] && grep -q "127.0.0.1:${FAKE_NGINX_BAD_PORT}" "$UPSTREAM_FILE"; then
  exit 1
fi
exit 0
""",
    "systemctl": """#!/usr/bin/env bash
echo "systemctl $*" >>"$FAKE_LOG"
""",
    "hostname": """#!/usr/bin/env bash
echo "${FAKE_HOSTNAME:-online-staging}"
""",
    "sleep": "#!/usr/bin/env bash\nexit 0\n",
}

LEGACY_UPSTREAM = "upstream quicc_online { server 127.0.0.1:1323; }\n"


def version_json(version="0.1.0", commit=SHA):
    return json.dumps({"commit": commit, "service": "quicc-online", "version": version})


@pytest.fixture
def box(tmp_path):
    """A fake staging box: deploy dir, nginx conf dir, fake bin dir."""
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    for name, body in FAKES.items():
        p = bin_dir / name
        p.write_text(body)
        p.chmod(p.stat().st_mode | stat.S_IEXEC)

    deploy_dir = tmp_path / "deploy"
    deploy_dir.mkdir()
    shutil.copy(DEPLOY, deploy_dir / "deploy.sh")
    shutil.copy(ROOT / "deploy" / "docker-compose.staging.yml", deploy_dir)
    (deploy_dir / ".env.stag").write_text("BLUE_TAG=staging-aaaaaaa\nGREEN_TAG=staging-bbbbbbb\n")
    (deploy_dir / ".state").write_text("blue\n")

    conf = tmp_path / "nginx"
    conf.mkdir()
    upstream = conf / "quicc-online-upstream.conf"
    upstream.write_text(LEGACY_UPSTREAM)

    log = tmp_path / "fake.log"
    log.write_text("")
    env = {
        "PATH": f"{bin_dir}:{os.environ['PATH']}",
        "HOME": str(tmp_path),
        "FAKE_LOG": str(log),
        "FAKE_APP_ENV": "staging",
        "FAKE_SLOT_JSON": version_json(),
        "FAKE_NGINX_JSON": version_json(),
        "UPSTREAM_FILE": str(upstream),
        "PROD_MARKER_DIR": str(tmp_path / "no-such-dir"),
        "SUDO": "",
        "HEALTH_RETRIES": "2",
    }

    class Box:
        dir = deploy_dir
        upstream_file = upstream

        def run(self, tag=f"staging-{SHA[:7]}", **extra):
            e = dict(env, **{k: str(v) for k, v in extra.items()})
            return subprocess.run(
                ["bash", str(deploy_dir / "deploy.sh"), tag],
                env=e, cwd=tmp_path, capture_output=True, text=True, timeout=60,
            )

        def log(self):
            return log.read_text()

        def state(self):
            return (deploy_dir / ".state").read_text().strip()

        def env_file(self):
            return (deploy_dir / ".env.stag").read_text()

    return Box()


def out(r):
    return r.stdout + r.stderr


# ── happy paths ──────────────────────────────────────────────────────────


def test_dry_run_prints_the_plan_and_changes_nothing(box):
    r = box.run(DRY_RUN=1)
    assert r.returncode == 0, out(r)
    o = r.stdout
    order = [
        "DRY: set 'GREEN_TAG=staging-1a2b3c4' in .env.stag",
        "--profile green pull online-green",
        "--profile green up -d --no-deps --force-recreate online-green",
        "DRY: write " + str(box.upstream_file) + ": upstream quicc_online { server 127.0.0.1:13232; }",
        "DRY: systemctl reload nginx",
        "--profile blue stop online-blue",
        "DRY: write green to .state",
    ]
    pos = [o.find(s) for s in order]
    assert all(p >= 0 for p in pos), f"missing step in:\n{o}\n{pos}"
    assert pos == sorted(pos), f"steps out of order:\n{o}"
    # Nothing changed, nothing mutating actually ran.
    assert box.state() == "blue"
    assert "GREEN_TAG=staging-bbbbbbb" in box.env_file()
    assert box.upstream_file.read_text() == LEGACY_UPSTREAM
    assert "docker " not in box.log()
    assert "systemctl" not in box.log()


def test_deploy_switches_slot(box):
    r = box.run()
    assert r.returncode == 0, out(r)
    assert box.state() == "green"
    assert "GREEN_TAG=staging-1a2b3c4" in box.env_file()
    assert "BLUE_TAG=staging-aaaaaaa" in box.env_file()
    assert "server 127.0.0.1:13232;" in box.upstream_file.read_text()
    assert Path(str(box.upstream_file) + ".bak").read_text() == LEGACY_UPSTREAM
    log = box.log()
    assert "up -d --no-deps --force-recreate online-green" in log
    assert "stop online-blue" in log
    assert "systemctl reload nginx" in log
    assert "https://staging.quiccpos.com/api/v1/version" in log
    # And back again: green → blue.
    r = box.run(tag=f"staging-{SHA[:8]}")
    assert r.returncode == 0, out(r)
    assert box.state() == "blue"
    assert "server 127.0.0.1:13231;" in box.upstream_file.read_text()


def test_first_deploy_without_state_goes_to_blue(box):
    (box.dir / ".state").unlink()
    r = box.run()
    assert r.returncode == 0, out(r)
    assert box.state() == "blue"
    assert "server 127.0.0.1:13231;" in box.upstream_file.read_text()
    assert " stop " not in box.log()
    assert "stop the old single-slot online container by hand" in r.stdout


def test_semver_tag_checks_version(box):
    r = box.run(tag="v0.2.0", FAKE_SLOT_JSON=version_json("0.2.0"), FAKE_NGINX_JSON=version_json("0.2.0"))
    assert r.returncode == 0, out(r)
    r = box.run(tag="v0.3.0", FAKE_SLOT_JSON=version_json("0.2.0"))
    assert r.returncode == 1


# ── failures roll back ───────────────────────────────────────────────────


def assert_rolled_back(box, r):
    assert r.returncode == 1, out(r)
    assert "rolling back" in r.stderr
    assert box.state() == "blue"
    assert "GREEN_TAG=staging-bbbbbbb" in box.env_file()
    assert box.upstream_file.read_text() == LEGACY_UPSTREAM
    assert "stop online-green" in box.log()
    assert "stop online-blue" not in box.log()


def test_wrong_version_on_new_slot(box):
    r = box.run(FAKE_SLOT_JSON=version_json(commit="ffffffff"))
    assert_rolled_back(box, r)
    assert "systemctl reload nginx" not in box.log()


def test_unhealthy_new_slot(box):
    r = box.run(FAKE_HEALTH="fail")
    assert_rolled_back(box, r)
    assert "never became healthy" in r.stderr


def test_nginx_config_test_fails(box):
    r = box.run(FAKE_NGINX_BAD_PORT=13232)
    assert_rolled_back(box, r)
    assert "nginx -t failed" in r.stderr


def test_nginx_still_serves_old_version(box):
    r = box.run(FAKE_NGINX_JSON=version_json(commit="0000000"))
    assert_rolled_back(box, r)
    # reloaded onto the new slot, then again onto the restored upstream
    assert box.log().count("systemctl reload nginx") == 2


def test_docker_failure(box):
    r = box.run(FAKE_DOCKER_RC=1)
    assert_rolled_back(box, r)


# ── refusals ─────────────────────────────────────────────────────────────


@pytest.mark.parametrize(
    "extra, why",
    [
        ({"FAKE_HOSTNAME": "lightsail-api"}, "prod denylist"),
        ({"FAKE_HOSTNAME": "ingest-1"}, "prod denylist"),
        ({"FAKE_APP_ENV": "prod"}, "not staging"),
        ({"FAKE_APP_ENV": ""}, "not staging"),
        ({"PUBLIC_HOST": "ingest.quiccpos.com"}, "prod host"),
        ({"PROD_MARKER_DIR": "/"}, "looks like the prod online box"),
    ],
)
def test_refuses_prod(box, extra, why):
    r = box.run(**extra)
    assert r.returncode == 1
    assert "REFUSING" in r.stderr and why in r.stderr, out(r)
    assert "docker " not in box.log()
    assert box.upstream_file.read_text() == LEGACY_UPSTREAM


def test_refuses_floating_tag(box):
    r = box.run(tag="staging-latest")
    assert r.returncode == 1
    assert "pinned tag" in r.stderr
    assert "docker " not in box.log()


def test_floating_tag_with_expectation(box):
    r = box.run(tag="staging-latest", EXPECT_COMMIT=SHA[:7])
    assert r.returncode == 0, out(r)


def test_needs_one_time_nginx_setup(box):
    box.upstream_file.unlink()
    r = box.run()
    assert r.returncode == 1
    assert "one-time nginx setup" in r.stderr
    assert "docker " not in box.log()


def test_deploy_script_syntax():
    subprocess.run(["bash", "-n", str(DEPLOY)], check=True)
    subprocess.run(["bash", "-n", str(BACKUP)], check=True)


# ── backup.sh ────────────────────────────────────────────────────────────


@pytest.mark.skipif(shutil.which("sqlite3") is None, reason="needs the sqlite3 CLI")
def test_backup_keeps_n_checked_copies(tmp_path):
    db = tmp_path / "app.db"
    con = sqlite3.connect(db)
    con.execute("PRAGMA journal_mode=WAL")
    con.execute("CREATE TABLE api_keys (id TEXT PRIMARY KEY, key TEXT, status TEXT)")
    con.execute("INSERT INTO api_keys VALUES ('a', 'k', 'active')")
    con.commit()
    dest = tmp_path / "bk"
    env = dict(os.environ, ONLINE_DB=str(db), ONLINE_BACKUP_DIR=str(dest), ONLINE_BACKUP_KEEP="2")
    for i in range(3):
        # distinct timestamps without sleeping: pre-create older copies
        (dest).mkdir(exist_ok=True)
        r = subprocess.run(["bash", str(BACKUP)], env=env, capture_output=True, text=True, timeout=60)
        assert r.returncode == 0, r.stdout + r.stderr
        assert "integrity ok, active keys: 1" in r.stdout
        newest = sorted(dest.glob("online-*.db"))[-1]
        newest.rename(dest / f"online-2026010{i}T000000Z.db")
    copies = sorted(p.name for p in dest.glob("online-*.db"))
    assert len(copies) <= 3
    r = subprocess.run(["bash", str(BACKUP)], env=env, capture_output=True, text=True, timeout=60)
    assert r.returncode == 0
    copies = sorted(p.name for p in dest.glob("online-*.db"))
    assert len(copies) == 2, copies
    assert oct(dest.stat().st_mode)[-3:] == "700"
    con.close()
    check = sqlite3.connect(dest / copies[-1])
    assert check.execute("SELECT key FROM api_keys WHERE status='active'").fetchone() == ("k",)
    check.close()
