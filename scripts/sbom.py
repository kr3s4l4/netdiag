#!/usr/bin/env python3
"""Genera un SBOM CycloneDX determinista a partir de `go list -m -json all`.

No usa red. Si SOURCE_DATE_EPOCH está definido, se usa como timestamp y como
parte de la semilla del serialNumber, de modo que el mismo árbol de módulos
produzca siempre el mismo SBOM.
"""
import hashlib
import json
import os
import subprocess
import sys
import uuid
from datetime import datetime, timezone

CYCLONEDX_NS = uuid.UUID("6f4b1b6d-3f0a-4f3d-9e2a-9d6e1c9a1f22")


def decode_stream(text):
    decoder = json.JSONDecoder()
    pos = 0
    while pos < len(text):
        while pos < len(text) and text[pos].isspace():
            pos += 1
        if pos >= len(text):
            break
        obj, pos = decoder.raw_decode(text, pos)
        yield obj


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else "sbom.cdx.json"
    version = os.environ.get("NETDIAG_VERSION", "dev")
    epoch = os.environ.get("SOURCE_DATE_EPOCH", "")
    if epoch:
        timestamp = datetime.fromtimestamp(int(epoch), timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    else:
        timestamp = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    raw = subprocess.check_output(["go", "list", "-m", "-json", "all"], text=True)
    components = []
    for module in decode_stream(raw):
        path = module.get("Path", "")
        if not path:
            continue
        component = {
            "type": "library",
            "bom-ref": f"pkg:golang/{path}@{module.get('Version', '')}",
            "name": path,
            "version": module.get("Version", ""),
            "purl": f"pkg:golang/{path}@{module.get('Version', '')}",
        }
        licence = module.get("License", "")
        if licence and licence not in ("", "UNKNOWN"):
            component["licenses"] = [{"license": {"id": licence}}]
        if module.get("Replace"):
            component["properties"] = [
                {"name": "replacement", "value": module["Replace"].get("Path", "")}
            ]
        components.append(component)
    components.sort(key=lambda c: c["name"])

    go_version = subprocess.check_output(["go", "version"], text=True).strip()
    bom = {
        "bomFormat": "CycloneDX",
        "specVersion": "1.5",
        "version": 1,
        "metadata": {
            "timestamp": timestamp,
            "tools": [{"vendor": "Go project", "name": "go", "version": go_version}],
            "component": {"type": "application", "name": "netdiag", "version": version},
        },
        "components": components,
    }
    canonical = json.dumps(bom, sort_keys=True, separators=(",", ":"), ensure_ascii=True)
    serial = uuid.uuid5(CYCLONEDX_NS, hashlib.sha256(canonical.encode()).hexdigest())
    bom["serialNumber"] = f"urn:uuid:{serial}"
    with open(out, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(bom, handle, sort_keys=True, indent=2, ensure_ascii=True)
        handle.write("\n")
    print(f"SBOM: {out} ({len(components)} módulos)")


if __name__ == "__main__":
    main()
