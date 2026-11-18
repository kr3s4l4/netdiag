#!/usr/bin/env python3
"""Genera una atestación de procedencia determinista para un release.

No es una firma: la atestación se firma aparte con minisign. Recoge el commit,
el tag, la versión de Go, SOURCE_DATE_EPOCH y los hashes de go.mod/go.sum para
que un tercero pueda repetir la construcción.
"""
import hashlib
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

VAULT = "https://updates.netdiag.example"
ROOT = Path(__file__).resolve().parent.parent


def git(*args, default=""):
    try:
        return subprocess.check_output(["git", *args], text=True, stderr=subprocess.DEVNULL).strip()
    except subprocess.CalledProcessError:
        return default


def sha256(path):
    with open(path, "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else "provenance.intoto.jsonl"
    version = os.environ.get("NETDIAG_VERSION", "dev")
    epoch = os.environ.get("SOURCE_DATE_EPOCH", "")
    timestamp = (
        datetime.fromtimestamp(int(epoch), timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        if epoch
        else datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    )
    statement = {
        "_type": "https://in-toto.io/Statement/v0.1",
        "subject": [
            {"name": "netdiag-linux-amd64", "digest": {"sha256": sha256("netdiag-linux-amd64")}},
            {"name": "netdiag-linux-arm64", "digest": {"sha256": sha256("netdiag-linux-arm64")}},
        ],
        "predicateType": "https://slsa.dev/provenance/v0.2",
        "predicate": {
            "builder": {"id": VAULT + "/builder/reproducible-go"},
            "buildType": "reproducible-go-build",
            "invocation": {
                "configSource": {
                    "uri": "https://github.com/kr3s4l4/netdiag",
                    "digest": {"sha1": git("rev-parse", "HEAD")},
                },
                "parameters": {
                    "version": version,
                    "commit": git("rev-parse", "HEAD"),
                    "tag": git("describe", "--tags", "--exact-match"),
                    "go_version": subprocess.check_output(["go", "version"], text=True).strip(),
                    "source_date_epoch": epoch,
                    "cgo_enabled": "0",
                    "flags": "-trimpath -buildvcs=false -s -w",
                },
            },
            "materials": [
                {"uri": "go.mod", "digest": {"sha256": sha256(ROOT / "go.mod")}},
                {"uri": "go.sum", "digest": {"sha256": sha256(ROOT / "go.sum")}},
            ],
            "metadata": {"buildFinishedOn": timestamp},
        },
    }
    with open(out, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps(statement, sort_keys=True, separators=(",", ":")) + "\n")
    print(f"procedencia: {out}")


if __name__ == "__main__":
    main()
