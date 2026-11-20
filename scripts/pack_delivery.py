#!/usr/bin/env python3
"""Build a manual delivery ZIP: signed artifacts + one signed licence.

No network, no telemetry, no signing keys. The licence file is supplied by the
issuer and is copied verbatim into the package.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import zipfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
PUBLIC_KEY = (
    "RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY"
)
FIXED_TIME = (1980, 1, 1, 0, 0, 0)

RELEASE_FILES = [
    "netdiag-linux-amd64",
    "netdiag-linux-arm64",
    "SHA256SUMS",
    "SHA256SUMS.minisig",
    "manifest.json",
    "manifest.json.minisig",
    "RELEASE_NOTES.md",
    "RELEASE_NOTES.md.minisig",
    "sbom.cdx.json",
    "provenance.intoto.jsonl",
    "release.pub",
]

DOCS = [
    "README.md",
    "LICENSE",
    "NOTICE",
    "SECURITY.md",
    "THIRD_PARTY_LICENSES.md",
    "docs/public_keys.md",
    "docs/publicacion-segura.md",
    "docs/licenciamiento.md",
    "docs/despliegue.md",
    "docs/plantillas/guia-endurecimiento.md",
]


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def read_release_version(release: pathlib.Path) -> str:
    raw = (release / "manifest.json").read_text(encoding="utf-8")
    try:
        value = json.loads(raw).get("version", "")
    except json.JSONDecodeError:
        value = ""
    if not value:
        raise SystemExit("manifest.json no contiene un campo version válido")
    return str(value)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--release-dir", required=True, help="directorio de release firmado")
    ap.add_argument("--license", required=True, help="license.json firmada por el emisor")
    ap.add_argument("--out", required=True, help="ZIP de salida")
    args = ap.parse_args()

    release = pathlib.Path(args.release_dir).resolve()
    license_path = pathlib.Path(args.license).resolve()
    out = pathlib.Path(args.out).resolve()

    missing = [name for name in RELEASE_FILES if not (release / name).is_file()]
    if missing:
        raise SystemExit("faltan artefactos de release: " + ", ".join(missing))
    if not license_path.is_file():
        raise SystemExit(f"no existe la licencia: {license_path}")

    version = read_release_version(release)
    entries: list[tuple[str, bytes]] = []

    for name in RELEASE_FILES:
        entries.append((name, (release / name).read_bytes()))

    ndc = release / f"netdiag_{version}_linux_amd64.ndc"
    if ndc.is_file():
        entries.append((ndc.name, ndc.read_bytes()))

    entries.append(("license.json", license_path.read_bytes()))

    for rel in DOCS:
        src = ROOT / rel
        if src.is_file():
            entries.append((rel, src.read_bytes()))

    lines = [
        "NetDiag — paquete de entrega manual",
        f"Versión: {version}",
        "Licencia: license.json (firmada por el emisor, sin servidor de activación)",
        "",
        "Contenido y SHA256:",
    ]
    for name, data in sorted(entries, key=lambda item: item[0]):
        lines.append(f"  {sha256(data)}  {name}")
    lines.extend(
        [
            "",
            "Este ZIP no contiene claves privadas ni semillas de firma.",
        ]
    )
    entries.append(("ENTREGA.txt", ("\n".join(lines) + "\n").encode("utf-8")))

    readme = f"""NetDiag {version} — primeros pasos

1. Verifica la descarga antes de ejecutar nada:

   minisign -V -P '{PUBLIC_KEY}' \\
     -m SHA256SUMS -x SHA256SUMS.minisig
   sha256sum -c SHA256SUMS

2. Activa la licencia en un directorio de datos propio:

   chmod +x netdiag-linux-amd64
   ./netdiag-linux-amd64 license activate \\
     --data-dir ./netdiag-data license.json

3. Comprueba el resultado:

   ./netdiag-linux-amd64 license show --data-dir ./netdiag-data

4. Inicia el servidor:

   ./netdiag-linux-amd64 serve --data-dir ./netdiag-data

La licencia se verifica localmente. NetDiag no contacta con ningún servidor.
Si la licencia caduca o se excede la banda, aparece un aviso; los datos no se
borran y las lecturas siguen funcionando.
"""
    entries.append(("LEEME.txt", readme.encode("utf-8")))

    out.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as zf:
        for name, data in entries:
            info = zipfile.ZipInfo(name, date_time=FIXED_TIME)
            info.compress_type = zipfile.ZIP_DEFLATED
            mode = 0o755 if name in {"netdiag-linux-amd64", "netdiag-linux-arm64"} else 0o644
            info.external_attr = (mode & 0xFFFF) << 16
            zf.writestr(info, data)

    print(f"Paquete: {out}")
    print(f"Ficheros: {len(entries)}")
    print(f"SHA256: {sha256(out.read_bytes())}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
