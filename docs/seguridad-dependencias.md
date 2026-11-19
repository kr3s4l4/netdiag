# Detección de vulnerabilidades en dependencias

NetDiag **no envía** avisos a ENISA, al CSIRT ni a ningún servicio externo. Esta configuración solo detecta CVEs y avisa al responsable local.

## GitHub

`.github/dependabot.yml` revisa el módulo Go una vez por semana y abre PRs con las etiquetas `dependencies` y `security`. Cada PR debe revisarse antes de fusionarse.

## Local

```bash
bash scripts/govulncheck.sh
```

Si la herramienta no está instalada:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

El script encuentra `govulncheck` en `PATH` o en `$(go env GOPATH)/bin`. Solo consulta la base de datos de vulnerabilidades; no envía datos del proyecto a ningún servicio.

## Toolchain

Usar **Go 1.27.4 o superior**. Go 1.27.3 arrastra `GO-2026-6859` (`net/http`, corregida en 1.27.4). El 19/11/2026 el escaneo con 1.27.4 devolvió `No vulnerabilities found`.

```bash
GOTOOLCHAIN=go1.27.4 bash scripts/govulncheck.sh
```

## Triaje

1. Confirmar la versión afectada contra el SBOM de la release.
2. Decidir si hay explotación activa o incidente grave.
3. Si la respuesta es sí, seguir el runbook del F41 con responsable humano.
4. Nunca automatizar el envío del aviso: la decisión y el justificante son manuales.

Un hallazgo de dependencia no implica por sí solo un incidente grave. La gravedad y el plazo se evalúan antes de reportar.
