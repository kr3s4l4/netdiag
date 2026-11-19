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

## Triaje

1. Confirmar la versión afectada contra el SBOM de la release.
2. Decidir si hay explotación activa o incidente grave.
3. Si la respuesta es sí, seguir el runbook del F41 con responsable humano.
4. Nunca automatizar el envío del aviso: la decisión y el justificante son manuales.

Un hallazgo de dependencia no implica por sí solo un incidente grave. La gravedad y el plazo se evalúan antes de reportar.
