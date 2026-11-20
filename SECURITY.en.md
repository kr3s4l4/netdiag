# NetDiag security policy

> Spanish version: [SECURITY.md](SECURITY.md).

## Supported versions

| Version | Security support |
| --- | --- |
| 1.5.x | Yes |
| 1.4.x | Until the next minor, critical fixes only |
| < 1.4 | No |

**No contractual window:** NetDiag is free software. The table above describes the security support offered. There is no SLA or formal support: issues are handled through GitHub Issues.

## Reporting a vulnerability

**Do not open a public issue** with exploitable details.

Use the repository's private GitHub Security Advisories channel:

<https://github.com/kr3s4l4/netdiag/security/advisories/new>

Include, if possible:

- Affected version and platform.
- Steps to reproduce, or a minimal proof of concept.
- Expected impact (confidentiality, integrity, availability).
- Relevant configuration, without real secrets.

## Response commitments

| Phase | Target |
| --- | --- |
| Acknowledgement | 3 business days |
| Triage and classification | 10 business days; immediately if there are signs of active exploitation |
| CRA notification when applicable | 24 h (early warning) and 72 h (full notification); **we do not wait for a patch** |
| Patch for an actively exploited critical vulnerability | target 72 h–7 days |
| Patch for a non-exploited high-severity issue | target 30 days |
| Medium/low severity | next release, target 90 days |
| Publication | signed notes, notice in this file and public notice |

If there is credible evidence of active exploitation or a serious incident, we report it even when no fix exists yet. The patch comes later; the notice cannot wait.

If the issue affects the signing key or the release pipeline, follow the revocation procedure in `docs/publicacion-segura.md` immediately.

## CRA incident runbook

**Single owner:** `[name, role, email, phone]`.  
**Deputy:** `[name, role, email, phone]`.  
**Account on the ENISA Single Reporting Platform:** `[organisation ID, user, credential manager]`. Verify access every quarter. Until these three fields are filled in, the runbook is not complete.

F41 target: early warning in **< 24 h**, official notification in **< 72 h** and a signed corrective release in **< 7 days**.

Decision tree:

```text
Active exploitation OR serious incident?
├─ YES → 24 h warning + 72 h notification. Do not wait for the patch.
│       Final report: 14 days after the patch if exploited;
│       1 month after the 72 h notification if it is a serious incident.
└─ NO → triage and fix in a signed release. Keep monitoring the criteria.
        If it becomes YES, switch to the upper branch immediately.
```

Steps:

1. Record the discovery time in UTC. Open `docs/plantillas/registro-incidentes.md` and `docs/plantillas/registro-responsabilidad.md`.
2. Decide whether to report. Record the reason and who approves the decision.
3. Send the [24 h warning](docs/plantillas/aviso-cra-24h.md) to the CSIRT of the member state of the main establishment and to the ENISA SRP. Keep the receipt.
4. Send the [72 h notification](docs/plantillas/aviso-cra-72h.md) with the initial assessment.
5. Fix in a new signed release with SBOM, manifest and `min_version` pointing to the first safe version. F23 marks those installations with `security_update: true` and emits `update.security`; the user sees the notice even when the version comparison does not look urgent.
6. Publish a public notice, show it in the UI and publish a signed [REVOCATION.md](docs/plantillas/REVOCATION.md) when applicable. In air-gapped environments deliver a signed `.ndc` with the notice.
7. Send the final report and close both records.

**Dependency monitoring:** every release ships a CycloneDX SBOM (`sbom.cdx.json`) and provenance attestation. Review advisories periodically. A relevant CVE is treated as an internal incident even without known exploitation. The producer is responsible for the finished product; upstream claims follow afterwards if a contract exists. **Never sign a third-party binary without rebuilding or verifying it.** See `docs/publicacion-segura.md`.

**Scan authorisation:** whoever runs NetDiag declares they are authorised to analyse those networks. Scanning third parties is prohibited.

## CRA obligations in the EU

Since **11 September 2026**, NetDiag as a product with digital elements is subject to the Cyber Resilience Act notification obligations (EU Regulation 2024/2847):

- **24 h:** early warning to the CSIRT of the member state of the main establishment and to ENISA through the Single Reporting Platform.
- **72 h:** full notification with initial assessment.
- **14 days after the patch:** final report for actively exploited vulnerabilities.
- **1 month after the 72 h notification:** final report for serious incidents.

This also applies to versions placed on the market before 11 December 2027. The remaining CRA obligations — technical documentation, conformity assessment, instructions for use, support period and marking where applicable — start on **11 December 2027**.

Responsibility lies with the **manufacturer**, i.e. whoever distributes NetDiag under their name or brand. A vulnerability in a third-party dependency does not transfer that responsibility to whoever deploys it: it is handled in our product and reported upstream where applicable.

## Disclosure

- We prefer coordinated disclosure: the reporter receives acknowledgement before publication.
- If there is no reply within 90 days, the reporter may disclose responsibly.
- We welcome public credit, but respect anonymity on request.

## Scope

- NetDiag code, embedded SPA, API and documentation.
- Build pipeline, release mirror and signing scheme.
- Dependencies in `go.mod` when they affect the published binary.

**Out of scope:** insecure configurations chosen by the operator, exposing an instance without TLS, credentials leaked by the user, attacks requiring root on the host running NetDiag, and purely theoretical reports with no impact scenario.

## Secure deployment reminder

- Do not expose NetDiag directly to the internet without authentication and HTTPS.
- Change the initial password and use least-privilege roles.
- Set `security.metrics_token` if `/metrics` is reachable outside loopback.
- Always verify the release signature before updating: `docs/publicacion-segura.md`.
- Keep automatic backups enabled and test restores.
