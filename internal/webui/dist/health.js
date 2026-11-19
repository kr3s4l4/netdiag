'use strict';

/* ============================================================
 * NetDiag SPA — F28: explainable network health panel.
 * Builds a 0–100 score from four weighted components and shows
 * the breakdown plus the three most relevant diagnostic findings.
 * Data comes from existing endpoints only: /dashboard,
 * /history/compare and /diagnostics/enterprise. No new calls of
 * its own, no external dependencies, no telemetry.
 * Depends on helpers from app.js: tr, esc, icon, fmtNum, SEV_LABELS.
 * ============================================================ */

const HEALTH_WEIGHTS = {
  availability: 40,
  latency: 20,
  changes: 20,
  severity: 20,
};
const HEALTH_ORDER = ['availability', 'latency', 'changes', 'severity'];
const HEALTH_SEV_RANK = { critical: 3, warning: 2, info: 1 };

function healthClamp(v, lo, hi) {
  return Math.max(lo, Math.min(hi, v));
}

function healthTone(score) {
  if (score === null) return { tone: 'muted', label: tr('Sin datos') };
  if (score >= 80) return { tone: 'green', label: tr('Saludable') };
  if (score >= 60) return { tone: 'amber', label: tr('Aceptable') };
  return { tone: 'red', label: tr('Atención requerida') };
}

// Returns null when there is no evidence for a component. Missing components
// do not score zero: their weight is removed and the rest are renormalised.
function computeHealthScore(stats, compare, diagnostics) {
  const s = stats || {};
  const cmp = compare || {};
  const pa = cmp.period_a && cmp.period_a.metrics ? cmp.period_a.metrics : null;
  const pb = cmp.period_b && cmp.period_b.metrics ? cmp.period_b.metrics : null;
  const diag = diagnostics || {};
  const counts = diag.counts || {};
  const anomalies = Array.isArray(diag.anomalies) ? diag.anomalies : [];
  const components = {};

  // Availability: direct percentage, 0–100.
  const availSamples = pa ? Number(pa.samples) || 0 : 0;
  const availHasData = availSamples > 0 || Number(s.availability) > 0;
  const availRaw = availSamples > 0 ? Number(pa.availability) : Number(s.availability);
  components.availability = {
    label: tr('Disponibilidad'),
    weight: HEALTH_WEIGHTS.availability,
    value: availHasData ? healthClamp(availRaw, 0, 100) : null,
    raw: availHasData ? fmtNum(availRaw, 1) + '%' : tr('Sin datos'),
  };

  // Latency: full marks at <= 10 ms, zero at >= 150 ms, linear between.
  const latSamples = pa ? Number(pa.samples) || 0 : 0;
  const latRaw = latSamples > 0 ? Number(pa.avg_rtt) : Number(s.avg_rtt);
  const latHasData = Number(latRaw) > 0 && (latSamples > 0 || Number(s.avg_rtt) > 0);
  components.latency = {
    label: tr('Latencia'),
    weight: HEALTH_WEIGHTS.latency,
    value: latHasData ? healthClamp(100 - ((Number(latRaw) - 10) / 140) * 100, 0, 100) : null,
    raw: latHasData ? fmtNum(Number(latRaw), 1) + ' ms' : tr('Sin datos'),
  };

  // Unexpected changes: the compare endpoint only reports threshold breaches,
  // so every change counts. 0 changes = 100, 10 or more = 0.
  const cmpHasData = !!(cmp.summary && typeof cmp.summary.total === 'number')
    && ((pa && Number(pa.samples) > 0) || Number(s.hosts_total) > 0);
  const changeCount = cmp.summary && Number(cmp.summary.total) ? Number(cmp.summary.total) : 0;
  components.changes = {
    label: tr('Cambios no esperados'),
    weight: HEALTH_WEIGHTS.changes,
    value: cmpHasData ? healthClamp(100 - changeCount * 10, 0, 100) : null,
    raw: cmpHasData ? String(changeCount) : tr('Sin datos'),
  };

  // Finding severity: critical weighs 25 points, warning 8 points.
  const diagHasData = !!diag.from || Object.keys(counts).length > 0
    || anomalies.length > 0 || diag.enterprise_mode !== undefined;
  const critical = Number(counts.critical) || 0;
  const warning = Number(counts.warning) || 0;
  components.severity = {
    label: tr('Severidad de hallazgos'),
    weight: HEALTH_WEIGHTS.severity,
    value: diagHasData ? healthClamp(100 - critical * 25 - warning * 8, 0, 100) : null,
    raw: diagHasData ? critical + ' críticas · ' + warning + ' avisos' : tr('Sin datos'),
  };

  const available = HEALTH_ORDER.map((k) => components[k]).filter((c) => c.value !== null);
  const totalWeight = available.reduce((sum, c) => sum + c.weight, 0);
  const score = totalWeight > 0
    ? Math.round(available.reduce((sum, c) => sum + c.value * c.weight, 0) / totalWeight)
    : null;

  let trend = null;
  if (pa && pb && Number(pa.samples) > 0 && Number(pb.samples) > 0) {
    trend = {
      availability: Number(pa.availability) - Number(pb.availability),
      latency: Number(pa.avg_rtt) - Number(pb.avg_rtt),
    };
  }

  const top = anomalies.slice().sort((a, b) => {
    const sa = HEALTH_SEV_RANK[a.severity] || 0;
    const sb = HEALTH_SEV_RANK[b.severity] || 0;
    if (sa !== sb) return sb - sa;
    return (Number(b.confidence) || 0) - (Number(a.confidence) || 0);
  }).slice(0, 3);

  return { score, tone: healthTone(score), components, order: HEALTH_ORDER, trend, top };
}

function healthDelta(value, unit) {
  if (value === null || value === undefined || !isFinite(Number(value))) return '';
  const v = Number(value);
  if (Math.abs(v) < 0.05) return '= 0 ' + unit;
  return (v > 0 ? '↑ ' : '↓ ') + fmtNum(Math.abs(v), 1) + ' ' + unit;
}

function healthTrendLine(trend) {
  if (!trend) return `<span class="muted">${esc(tr('Sin datos de tendencia'))}</span>`;
  return `${esc(healthDelta(trend.availability, 'pts'))} · ${esc(healthDelta(trend.latency, 'ms'))} <span class="muted">${esc(tr('vs hace 7 días'))}</span>`;
}

function healthComponentRow(c) {
  const pct = c.value === null ? 0 : healthClamp(c.value, 0, 100);
  const contribution = c.value === null
    ? tr('Sin datos')
    : fmtNum((c.value * c.weight) / 100, 1) + ' / ' + c.weight;
  const color = c.value === null ? 'var(--muted-2)'
    : c.value >= 80 ? 'var(--green)' : c.value >= 60 ? 'var(--amber)' : 'var(--red)';
  return `<div class="health-comp" title="${esc(c.raw)}">
    <span class="health-comp-label">${esc(c.label)} <small>${c.weight}%</small></span>
    <span class="health-bar"><i style="width:${pct.toFixed(1)}%;background:${color}"></i></span>
    <b class="mono health-comp-val">${esc(contribution)}</b>
    <span class="health-comp-raw">${esc(c.raw)}</span>
  </div>`;
}

function healthIncidentRow(a) {
  const sev = (typeof SEV_LABELS !== 'undefined' && SEV_LABELS[a.severity]) || a.severity || '';
  const evidence = Array.isArray(a.evidence) && a.evidence.length ? a.evidence[0] : '';
  const hostLink = Array.isArray(a.host_ids) && a.host_ids.length
    ? `<a class="tiny" href="#/hosts/${encodeURIComponent(a.host_ids[0])}">${esc(tr('Ver host'))}</a>` : '';
  return `<div class="health-incident">
    <span class="badge ${esc(a.severity || '')}">${esc(tr(sev))}</span>
    <div class="main">
      <b>${esc(a.title || (typeof DIAG_TYPE_LABELS !== 'undefined' && DIAG_TYPE_LABELS[a.type]) || a.type || '')}</b>
      <span class="tiny muted ellipsis">${esc(a.detail || '')}</span>
      ${evidence ? `<span class="tiny ellipsis">· ${esc(evidence)}</span>` : ''}
    </div>
    <div class="nowrap row-gap">${hostLink}
      <a class="btn sm ghost" href="#/diagnostics?finding=${encodeURIComponent(a.id || '')}">${esc(tr('Ver evidencia'))}</a>
    </div>
  </div>`;
}

function renderHealthCard(stats, compare, diagnostics) {
  const h = computeHealthScore(stats, compare, diagnostics);
  const scoreText = h.score === null ? '—' : String(h.score);
  const rows = h.order.map((k) => healthComponentRow(h.components[k])).join('');
  const trend = healthTrendLine(h.trend);
  const incidents = h.top.length
    ? h.top.map(healthIncidentRow).join('')
    : `<div class="empty">${icon('check')}<div><b>${esc(tr('Sin incidencias'))}</b>
        <div class="tiny">${esc(tr('Las heurísticas no encontraron señales suficientes.'))}</div></div></div>`;

  return `<div class="card pad mb health-card" id="health-card">
    <div class="spread mb">
      <div>
        <h3 style="margin:0;font-size:14px">${esc(tr('Salud de la red'))}</h3>
        <div class="tiny muted">${esc(tr('Puntuación explicable · ponderación 40/20/20/20 · últimos 24 h y comparativa a 7 días'))}</div>
      </div>
      <a class="btn sm" href="#/diagnostics">${icon('diagnostics')} ${esc(tr('Diagnósticos'))}</a>
    </div>
    <div class="health-main">
      <div class="health-score ${h.tone.tone}">
        <div><span class="health-number">${scoreText}</span><span class="health-max">/100</span></div>
        <div class="health-label">${esc(h.tone.label)}</div>
        <div class="health-trend">${trend}</div>
      </div>
      <div class="health-breakdown">
        ${rows}
      </div>
    </div>
    <div class="health-incidents">
      <div class="spread mb"><h4 style="margin:0;font-size:12.5px">${esc(tr('Incidencias principales'))}</h4>
        <a class="tiny" href="#/diagnostics">${esc(tr('Ver todas'))}</a></div>
      ${incidents}
    </div>
  </div>`;
}

window.NetDiagHealth = {
  compute: computeHealthScore,
  render: renderHealthCard,
};
