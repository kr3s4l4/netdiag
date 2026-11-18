'use strict';

/* ============================================================
 * NetDiag SPA — views: topology, alerts, scans, reports, settings.
 * Loaded before app.js; all symbols resolve at call time.
 * ============================================================ */

/* ------------------------------------------------------------------ */
/* Topology                                                            */
/* ------------------------------------------------------------------ */

const TOPO_KIND_COLORS = {
  network: '#a78bfa',
  gateway: '#fbbf24',
  router: '#38bdf8',
  switch: '#a78bfa',
  firewall: '#f87171',
  access_point: '#22d3ee',
  server: '#34d399',
  nas: '#2dd4bf',
  printer: '#fbbf24',
  camera: '#fb7185',
  phone: '#60a5fa',
  tablet: '#818cf8',
  laptop: '#93c5fd',
  workstation: '#93c5fd',
  virtual: '#c084fc',
  iot: '#facc15',
  media: '#f472b6',
  game_console: '#f472b6',
  unknown: '#8fa1b8',
};

function topoNodeColor(n) {
  if (n.kind === 'network') return TOPO_KIND_COLORS.network;
  if (n.is_gateway) return TOPO_KIND_COLORS.gateway;
  return TOPO_KIND_COLORS[n.device_type] || TOPO_KIND_COLORS.unknown;
}

function topoNodeLabel(n) {
  if (n.kind === 'network') return n.label || n.network || 'Red';
  return n.label || n.ip || 'Dispositivo';
}

function resetTopology() {
  state.topoView = { x: 0, y: 0, k: 1 };
  applyTopoTransform();
}

function zoomTopology(delta) {
  const v = state.topoView;
  const k = Math.max(0.35, Math.min(4, v.k * (delta > 0 ? 1.25 : 0.8)));
  const cx = 500, cy = 350;
  v.x = cx - (cx - v.x) * (k / v.k);
  v.y = cy - (cy - v.y) * (k / v.k);
  v.k = k;
  applyTopoTransform();
}

function applyTopoTransform() {
  const layer = document.getElementById('topo-layer');
  if (!layer) return;
  const v = state.topoView;
  layer.setAttribute('transform', `translate(${v.x.toFixed(1)} ${v.y.toFixed(1)}) scale(${v.k.toFixed(3)})`);
  const badge = document.getElementById('topo-zoom-label');
  if (badge) badge.textContent = Math.round(v.k * 100) + '%';
}

function layoutTopology(graph) {
  if (!graph || !graph.nodes || !graph.nodes.length) return;
  const clusters = new Map();
  graph.nodes.forEach((n) => {
    const key = String(n.network_id || 0);
    if (!clusters.has(key)) clusters.set(key, []);
    clusters.get(key).push(n);
  });
  const keys = Array.from(clusters.keys());
  const netCount = keys.length;
  const ringRadius = netCount <= 1 ? 0 : Math.max(260, 90 * netCount);
  keys.forEach((key, idx) => {
    const nodes = clusters.get(key);
    const angle = (idx / Math.max(1, netCount)) * Math.PI * 2 - Math.PI / 2;
    const cx = netCount <= 1 ? 0 : Math.cos(angle) * ringRadius;
    const cy = netCount <= 1 ? 0 : Math.sin(angle) * ringRadius * 0.62;
    const anchor = nodes.find((n) => n.kind === 'network') || nodes.find((n) => n.is_gateway);
    const rest = nodes.filter((n) => n !== anchor);
    if (anchor) {
      anchor.x = cx;
      anchor.y = cy;
    }
    const count = rest.length;
    const radius = count <= 0 ? 0 : Math.min(390, 74 + Math.sqrt(count) * 36);
    rest.forEach((n, i) => {
      const a = (i / Math.max(1, count)) * Math.PI * 2 + (Number(key) % 9) * 0.31;
      const rr = count === 1 ? 0 : radius * (0.62 + 0.38 * (((i * 37) % 100) / 100));
      n.x = cx + Math.cos(a) * rr;
      n.y = cy + Math.sin(a) * rr;
    });
  });
  let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
  graph.nodes.forEach((n) => {
    minX = Math.min(minX, n.x); maxX = Math.max(maxX, n.x);
    minY = Math.min(minY, n.y); maxY = Math.max(maxY, n.y);
  });
  const spanX = Math.max(1, maxX - minX);
  const spanY = Math.max(1, maxY - minY);
  const scale = Math.min(840 / spanX, 520 / spanY);
  const midX = (minX + maxX) / 2;
  const midY = (minY + maxY) / 2;
  graph.nodes.forEach((n) => {
    n.x = 500 + (n.x - midX) * scale;
    n.y = 350 + (n.y - midY) * scale;
  });
}

function drawTopology() {
  const graph = state.topo;
  const layer = document.getElementById('topo-layer');
  if (!graph || !layer) return;
  const positions = new Map(graph.nodes.map((n) => [n.id, n]));
  const degree = new Map();
  graph.edges.forEach((e) => {
    degree.set(e.source, (degree.get(e.source) || 0) + 1);
    degree.set(e.target, (degree.get(e.target) || 0) + 1);
  });
  const edgeHTML = graph.edges.map((e) => {
    const a = positions.get(e.source);
    const b = positions.get(e.target);
    if (!a || !b) return '';
    const cls = ['topo-edge', e.kind || 'arp'].join(' ');
    const dash = e.kind === 'subnet' ? ' stroke-dasharray="3 5"' : '';
    return `<line class="${cls}" x1="${a.x.toFixed(1)}" y1="${a.y.toFixed(1)}" x2="${b.x.toFixed(1)}" y2="${b.y.toFixed(1)}"${dash}>
      <title>${esc((e.kind || '').toUpperCase())}${e.label ? ' · ' + esc(e.label) : ''} · ${e.confidence || 0}%</title></line>`;
  }).join('');

  const labelEvery = graph.nodes.length > 70 ? 0 : 1;
  const nodeHTML = graph.nodes.map((n, idx) => {
    const color = topoNodeColor(n);
    const deg = degree.get(n.id) || 0;
    const isNet = n.kind === 'network';
    const r = isNet ? 14 : n.is_gateway ? 11 : Math.min(10, 6 + Math.sqrt(deg));
    const important = isNet || n.is_gateway || ['router', 'switch', 'firewall'].includes(n.device_type);
    const title = esc(topoNodeLabel(n) + (n.ip ? ' · ' + n.ip : '') + (n.vendor ? ' · ' + n.vendor : ''));
    const label = n.kind === 'network' ? topoNodeLabel(n) : (n.label || n.ip || '');
    const truncated = label.length > 26 ? label.slice(0, 25) + '…' : label;
    const showLabel = important || (labelEvery && idx % 2 === 0);
    const shape = isNet
      ? `<rect x="${(n.x - r).toFixed(1)}" y="${(n.y - r).toFixed(1)}" width="${r * 2}" height="${r * 2}" rx="4" fill="${color}" fill-opacity=".18" stroke="${color}" stroke-width="2"/>`
      : `<circle cx="${n.x.toFixed(1)}" cy="${n.y.toFixed(1)}" r="${r}" fill="${color}" fill-opacity=".16" stroke="${color}" stroke-width="2"/>
         <circle cx="${n.x.toFixed(1)}" cy="${n.y.toFixed(1)}" r="${Math.max(1.8, r * 0.28)}" fill="${color}"/>`;
    return `<g class="topo-node" data-host-id="${n.host_id || ''}" data-node-id="${esc(n.id)}" style="cursor:${n.host_id ? 'pointer' : 'default'}">
      ${shape}
      ${showLabel ? `<text class="label" x="${n.x.toFixed(1)}" y="${(n.y + r + 13).toFixed(1)}" text-anchor="middle">${esc(truncated)}</text>` : ''}
      <title>${title}</title>
    </g>`;
  }).join('');

  layer.innerHTML = `<g class="edges">${edgeHTML}</g><g class="nodes">${nodeHTML}</g>`;
  applyTopoTransform();
}

function setupTopoInteractions() {
  const svg = document.getElementById('topo-svg');
  if (!svg) return;
  let dragging = false;
  let startX = 0, startY = 0, originX = 0, originY = 0;
  const toView = (clientX, clientY) => {
    const rect = svg.getBoundingClientRect();
    return {
      x: ((clientX - rect.left) / rect.width) * 1000,
      y: ((clientY - rect.top) / rect.height) * 700,
    };
  };
  svg.addEventListener('pointerdown', (e) => {
    if (e.target.closest('.topo-node')) return;
    dragging = true;
    svg.classList.add('dragging');
    try { svg.setPointerCapture(e.pointerId); } catch (_) {}
    startX = e.clientX; startY = e.clientY;
    originX = state.topoView.x; originY = state.topoView.y;
  });
  svg.addEventListener('pointermove', (e) => {
    if (!dragging) return;
    const rect = svg.getBoundingClientRect();
    const sx = 1000 / rect.width;
    const sy = 700 / rect.height;
    state.topoView.x = originX + (e.clientX - startX) * sx;
    state.topoView.y = originY + (e.clientY - startY) * sy;
    applyTopoTransform();
  });
  const stop = () => { dragging = false; svg.classList.remove('dragging'); };
  svg.addEventListener('pointerup', stop);
  svg.addEventListener('pointercancel', stop);
  svg.addEventListener('click', (e) => {
    const node = e.target.closest('.topo-node');
    if (!node) return;
    const id = node.dataset.hostId;
    if (id) navTo('#/hosts/' + id);
  });
  svg.addEventListener('wheel', (e) => {
    e.preventDefault();
    const p = toView(e.clientX, e.clientY);
    const v = state.topoView;
    const factor = e.deltaY < 0 ? 1.12 : 0.89;
    const k = Math.max(0.35, Math.min(4, v.k * factor));
    v.x = p.x - (p.x - v.x) * (k / v.k);
    v.y = p.y - (p.y - v.y) * (k / v.k);
    v.k = k;
    applyTopoTransform();
  }, { passive: false });
}

async function renderTopology() {
  setPageTitle('topology');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const previous = state.topoView;
  const graph = await api('/topology');
  state.topo = graph;
  if (!graph.nodes || !graph.nodes.length) {
    content.innerHTML = `<div class="card"><div class="empty">
      ${icon('topology')}
      <div><b>Sin topología todavía</b></div>
      <div class="tiny" style="margin-top:6px">Ejecuta un escaneo con SNMP o ARP local para descubrir la estructura de la red.</div>
      <button class="btn primary mt" data-action="open-scan">${icon('radar')} Escanear ahora</button>
    </div></div>`;
    return;
  }
  layoutTopology(graph);
  state.topoView = previous || { x: 0, y: 0, k: 1 };
  const byType = graph.stats && graph.stats.by_type ? graph.stats.by_type : {};
  const typeSummary = Object.entries(byType)
    .sort((a, b) => b[1] - a[1]).slice(0, 5)
    .map(([k, v]) => `<span><i class="dot" style="background:${TOPO_KIND_COLORS[k] || 'var(--muted-2)'}"></i>${esc(typeLabel(k))} ${v}</span>`)
    .join('');
  content.innerHTML = `
    <div class="spread mb">
      <div class="kpi-mini">
        <div><b>${fmtNum(graph.stats.nodes)}</b><span>nodos</span></div>
        <div><b>${fmtNum(graph.stats.edges)}</b><span>enlaces</span></div>
        <div><b>${fmtNum(graph.stats.isolated)}</b><span>aislados</span></div>
      </div>
      <div class="row">
        <button class="btn sm" data-action="reload">${icon('refresh')} Actualizar</button>
        <button class="btn sm" data-action="topo-reset">Centrar</button>
      </div>
    </div>
    <div class="topo-wrap">
      <svg id="topo-svg" viewBox="0 0 1000 700" preserveAspectRatio="xMidYMid meet">
        <g id="topo-layer"></g>
      </svg>
      <div class="topo-tools">
        <button class="btn sm icon" data-action="topo-zoom" data-delta="1" title="Acercar">+</button>
        <button class="btn sm icon" data-action="topo-zoom" data-delta="-1" title="Alejar">−</button>
        <span class="btn sm" style="pointer-events:none" id="topo-zoom-label">100%</span>
      </div>
      <div class="topo-legend">
        <span><i class="dot" style="background:#a78bfa"></i>Red</span>
        <span><i class="dot" style="background:#fbbf24"></i>Puerta de enlace</span>
        <span><i class="dot" style="background:#38bdf8"></i>Router</span>
        <span><i class="dot" style="background:#34d399"></i>Servidor</span>
        <span><i class="dot" style="background:#8fa1b8"></i>Cliente</span>
        ${typeSummary}
      </div>
    </div>
    <div class="tiny muted mt">Arrastra para desplazar · rueda del ratón para zoom · clic en un dispositivo para ver su ficha</div>`;
  drawTopology();
  setupTopoInteractions();
}

/* ------------------------------------------------------------------ */
/* Enterprise diagnostics                                              */
/* ------------------------------------------------------------------ */

const DIAG_TYPE_LABELS = {
  stp_loop: 'Lazo STP',
  vlan_leak: 'Fuga de VLAN',
  duplex_mismatch: 'Desajuste de dúplex',
  mtu_problem: 'Problema de MTU',
  broadcast_storm: 'Tormenta de broadcast',
  rogue_ap: 'AP no autorizado',
  sla_breach: 'Incumplimiento de SLA',
};

function diagConfidenceBar(value) {
  const v = Math.max(0, Math.min(100, Number(value) || 0));
  const tone = v >= 85 ? 'var(--red)' : v >= 70 ? 'var(--amber)' : 'var(--accent)';
  return `<span class="bar" style="width:74px"><i style="width:${v}%;background:${tone}"></i></span>
    <span class="tiny mono">${v}%</span>`;
}

async function renderDiagnostics() {
  setPageTitle('diagnostics');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const report = await api('/diagnostics/enterprise');
  const anomalies = report.anomalies || [];
  const counts = report.counts || {};
  const windowLabel = `${fmtDate(report.from)} — ${fmtDate(report.to)}`;

  const rows = anomalies.length ? anomalies.map((a) => `
    <tr>
      <td><span class="badge ${esc(a.severity)}">${esc(tr(SEV_LABELS[a.severity] || a.severity))}</span></td>
      <td><div class="name-cell"><b>${esc(tr(DIAG_TYPE_LABELS[a.type] || a.type))}</b><span>${esc(a.detail)}</span></div></td>
      <td>${(a.host_ips || []).length ? (a.host_ips || []).slice(0, 3).map((ip) => `<span class="mono-ip">${esc(ip)}</span>`).join('<br>') : '<span class="muted">—</span>'}</td>
      <td>${diagConfidenceBar(a.confidence)}</td>
      <td class="tiny muted">${fmtDate(a.detected_at)}</td>
      <td class="right nowrap"><details class="diag-details"><summary>${esc(tr('Evidencia'))}</summary>
        <div class="diag-evidence">${(a.evidence || []).map((e) => `<div>· ${esc(e)}</div>`).join('')}
        ${a.recommendation ? `<div class="mt"><b>${esc(tr('Recomendación'))}:</b> ${esc(a.recommendation)}</div>` : ''}</div>
      </details></td>
    </tr>`).join('') : `<tr><td colspan="6"><div class="empty">${icon('check')}<div><b>${esc(tr('Sin anomalías detectadas'))}</b>
      <div class="tiny">${esc(tr('Las heurísticas no encontraron señales suficientes en la ventana analizada.'))}</div></div></div></td></tr>`;

  const categoryCounts = ['stp', 'vlan', 'duplex', 'performance', 'wireless', 'sla']
    .map((k) => [k, counts[k] || 0]).filter(([, v]) => v > 0);
  const hasEnterprise = report.enterprise_mode !== false;

  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">${esc(tr('Análisis heurístico a partir de vecindades LLDP/CDP/FDB, IF-MIB, inventario y muestras de disponibilidad. Cada hallazgo incluye evidencia, confianza y limitaciones.'))}</div>
      <button class="btn sm" data-action="refresh-diagnostics">${icon('refresh')} ${esc(tr('Actualizar'))}</button>
    </div>
    <div class="grid cols-4 mb">
      ${statCard({ label: tr('Anomalías'), value: fmtNum(anomalies.length), hint: `${fmtNum(report.hosts_analyzed)} ${tr('hosts analizados')}`, icon: 'diagnostics', tone: 'violet' })}
      ${statCard({ label: tr('Críticas'), value: fmtNum(counts.critical || 0), hint: tr('requieren atención inmediata'), icon: 'warn', tone: 'red' })}
      ${statCard({ label: tr('Avisos'), value: fmtNum(counts.warning || 0), hint: tr('requieren revisión'), icon: 'alerts', tone: 'amber' })}
      ${statCard({ label: tr('Muestras'), value: fmtNum(report.samples_analyzed), hint: tr('en la ventana'), icon: 'clock', tone: 'accent' })}
    </div>
    ${!hasEnterprise ? `<div class="notice mb">${icon('info')}<div><b>${esc(tr('Modo empresarial desactivado'))}</b>
      <div class="tiny">${esc(tr('Solo se evalúan los umbrales de SLA; configura diagnostics.enterprise_mode: true para el resto de detectores.'))}</div></div></div>` : ''}
    <div class="card mb">
      <div class="card-head"><h3>${esc(tr('Hallazgos'))}</h3><span class="tiny muted">${esc(windowLabel)}</span></div>
      <div class="table-wrap"><table>
        <thead><tr><th>${esc(tr('Severidad'))}</th><th>${esc(tr('Hallazgo'))}</th><th>${esc(tr('Hosts'))}</th><th>${esc(tr('Confianza'))}</th><th>${esc(tr('Detectado'))}</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
      </table></div>
    </div>
    ${categoryCounts.length ? `<div class="card pad mb"><div class="spread mb"><h3 style="margin:0;font-size:13.5px">${esc(tr('Por categoría'))}</h3></div>
      <div class="kpi-mini">${categoryCounts.map(([k, v]) => `<div><b>${v}</b><span>${esc(tr(k))}</span></div>`).join('')}</div></div>` : ''}
    <div class="card pad">
      <div class="spread mb"><h3 style="margin:0;font-size:13.5px">${esc(tr('Limitaciones del análisis'))}</h3><span class="badge info">${esc(tr('Heurístico'))}</span></div>
      <ul class="diag-limits">${(report.limitations || []).map((l) => `<li>${esc(l)}</li>`).join('')}</ul>
      <div class="tiny muted mt">${esc(tr('Un hallazgo es una pista para investigar, no una confirmación del protocolo. Confirma siempre en el equipo de red.'))}</div>
    </div>`;
}

/* ------------------------------------------------------------------ */
/* F21 temporal comparison                                             */
/* ------------------------------------------------------------------ */

const COMPARE_CHANGE_LABELS = {
  'host.added': 'Dispositivo nuevo',
  'host.removed': 'Dispositivo ausente',
  'host.status_changed': 'Cambio de estado',
  'host.availability_changed': 'Cambio de disponibilidad',
  'host.rtt_changed': 'Cambio de latencia',
  'topology.node_added': 'Nodo nuevo',
  'topology.node_removed': 'Nodo eliminado',
  'topology.edge_added': 'Enlace nuevo',
  'topology.edge_removed': 'Enlace eliminado',
  'fleet.availability_changed': 'Disponibilidad global',
  'fleet.rtt_changed': 'Latencia global',
};

function compareValue(value) {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'number') return Number.isInteger(value) ? String(value) : value.toFixed(2);
  return String(value);
}

function comparePeriodCard(period, tone, maxRTT) {
  const m = period.metrics || {};
  const rtt = period.rtt || [];
  const snap = period.topology && period.topology.snapshot_at;
  return `<div class="card">
    <div class="card-head"><h3>${esc(period.label || tr('Periodo'))}</h3>
      <span class="badge ${tone}">${esc(fmtDate(period.from))} → ${esc(fmtDate(period.to))}</span>
    </div>
    <div class="card-body">
      <div class="kpi-mini mb">
        <div><b>${fmtNum(m.samples || 0)}</b><span>${esc(tr('muestras'))}</span></div>
        <div><b>${fmtNum(m.availability || 0, 2)}%</b><span>${esc(tr('disponibilidad'))}</span></div>
        <div><b>${fmtNum(m.avg_rtt || 0, 1)} ms</b><span>${esc(tr('RTT medio'))}</span></div>
        <div><b>${fmtNum(m.hosts_up || 0)}</b><span>${esc(tr('activos'))}</span></div>
        <div><b>${fmtNum(m.hosts_down || 0)}</b><span>${esc(tr('caídos'))}</span></div>
      </div>
      <div class="tiny muted mb">${esc(tr('Latencia (ms)'))}${snap ? ` · ${esc(tr('topología'))}: ${esc(fmtDate(snap))}` : ''}</div>
      ${lineChart(rtt, { height: 170, min: 0, max: maxRTT, color: tone === 'up' ? 'var(--green)' : 'var(--violet)' })}
    </div>
  </div>`;
}

async function renderHistoryCompare() {
  setPageTitle('compare');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const report = await api('/history/compare', {
    method: 'POST',
    body: { days: state.compareDays || 7, window: state.compareWindow || '24h' },
  });
  const a = report.period_a || {};
  const b = report.period_b || {};
  const maxRTT = Math.max(10, Number(a.metrics && a.metrics.max_rtt) || 0, Number(b.metrics && b.metrics.max_rtt) || 0) * 1.1;
  const summary = report.summary || {};

  const dayButtons = [[1, '24 h'], [7, '7 días'], [30, '30 días']].map(([value, label]) =>
    `<button class="btn sm ${Number(state.compareDays) === value ? 'primary' : ''}" data-action="compare-run" data-days="${value}">${esc(tr(label))}</button>`).join('');

  const rows = (report.changes || []).length ? report.changes.map((c) => {
    const label = COMPARE_CHANGE_LABELS[c.type] || c.type;
    const evidence = (c.evidence || []).map((e) => `<div>· ${esc(e)}</div>`).join('');
    return `<tr>
      <td><span class="badge ${c.type.indexOf('removed') >= 0 || c.type.indexOf('down') >= 0 ? 'critical' : 'info'}">${esc(tr(label))}</span></td>
      <td class="tiny">${esc(c.entity)}</td>
      <td class="tiny">${esc(c.label || c.id)}</td>
      <td class="mono tiny">${esc(compareValue(c.from))}</td>
      <td class="mono tiny">${esc(compareValue(c.to))}</td>
      <td class="tiny muted">${evidence || '—'}</td>
    </tr>`;
  }).join('') : `<tr><td colspan="6"><div class="empty">${icon('check')}<div><b>${esc(tr('Sin cambios entre los periodos'))}</b>
    <div class="tiny">${esc(tr('Las métricas y la topología disponibles no muestran diferencias con los umbrales configurados.'))}</div></div></div></td></tr>`;

  const limitations = (report.limitations || []).length ? `<div class="notice warn mb">
      ${icon('warn')}<div><b>${esc(tr('Limitaciones del diff'))}</b>
      <ul class="diag-limits">${report.limitations.map((l) => `<li>${esc(l)}</li>`).join('')}</ul></div></div>` : '';

  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">${esc(tr('Compara dos momentos equivalentes de duración. Por defecto: hoy frente a la misma ventana hace 7 días.'))}</div>
      <div class="row-gap">
        ${dayButtons}
        <select class="select sm" data-action="compare-window" aria-label="${esc(tr('Ventana'))}">
          ${[['6h', '6 h'], ['24h', '24 h'], ['48h', '48 h'], ['7d', '7 días']].map(([v, l]) =>
            `<option value="${v}" ${state.compareWindow === v ? 'selected' : ''}>${esc(tr(l))}</option>`).join('')}
        </select>
      </div>
    </div>
    ${limitations}
    <div class="grid cols-2 compare-periods mb">
      ${comparePeriodCard(a, 'up', maxRTT)}
      ${comparePeriodCard(b, 'violet', maxRTT)}
    </div>
    <div class="card mb">
      <div class="card-head"><h3>${esc(tr('Cambios detectados'))}</h3>
        <span class="tiny muted">${fmtNum(summary.total || 0)} ${esc(tr('cambios'))}
          ${summary['host.added'] ? ` · ${summary['host.added']} ${esc(tr('nuevos'))}` : ''}
          ${summary['host.removed'] ? ` · ${summary['host.removed']} ${esc(tr('ausentes'))}` : ''}
          ${report.truncated ? ` · <span class="badge warning">${esc(tr('truncado'))}</span>` : ''}
        </span>
      </div>
      <div class="table-wrap"><table class="compare-changes">
        <thead><tr><th>${esc(tr('Tipo'))}</th><th>${esc(tr('Entidad'))}</th><th>${esc(tr('Objeto'))}</th>
          <th>${esc(tr('Antes'))}</th><th>${esc(tr('Ahora'))}</th><th>${esc(tr('Evidencia'))}</th></tr></thead>
        <tbody>${rows}</tbody>
      </table></div>
    </div>
    <div class="card pad">
      <div class="tiny muted">${esc(tr('Fuente: muestras de disponibilidad y latencia almacenadas; los cambios de topología usan snapshots capturados tras cada escaneo y cada hora. Un valor ausente se reporta como limitación, nunca como "sin cambios".'))}</div>
    </div>`;
}

/* ------------------------------------------------------------------ */
/* F22 audit mode                                                      */
/* ------------------------------------------------------------------ */

const AUDIT_CHANGE_LABELS = {
  'host.added': 'Dispositivo nuevo',
  'host.removed': 'Dispositivo ausente',
  'host.changed': 'Dispositivo modificado',
  'host.port_added': 'Puerto abierto',
  'host.port_removed': 'Puerto cerrado',
  'host.port_changed': 'Puerto modificado',
  'network.added': 'Red nueva',
  'network.removed': 'Red eliminada',
  'network.changed': 'Red modificada',
  'neighbor.added': 'Vecino nuevo',
  'neighbor.removed': 'Vecino eliminado',
  'neighbor.changed': 'Vecino modificado',
  'rule.added': 'Regla nueva',
  'rule.removed': 'Regla eliminada',
  'rule.changed': 'Regla modificada',
  'channel.added': 'Canal nuevo',
  'channel.removed': 'Canal eliminado',
  'channel.changed': 'Canal modificado',
  'integration.added': 'Integración nueva',
  'integration.removed': 'Integración eliminada',
  'integration.changed': 'Integración modificada',
  'scan.added': 'Escaneo nuevo',
  'scan.removed': 'Escaneo eliminado',
  'scan.changed': 'Escaneo modificado',
  'topology.node_added': 'Nodo nuevo',
  'topology.node_removed': 'Nodo eliminado',
  'topology.node_changed': 'Nodo modificado',
  'topology.edge_added': 'Enlace nuevo',
  'topology.edge_removed': 'Enlace eliminado',
  'topology.edge_changed': 'Enlace modificado',
  'alerts.changed': 'Alertas modificadas',
  'config.changed': 'Configuración modificada',
};

function auditSnapshotOption(snap, selected) {
  const label = `${snap.name || tr('Sin nombre')} · ${fmtDate(snap.ts)}`;
  return `<option value="${snap.id}" ${Number(selected) === snap.id ? 'selected' : ''}>${esc(label)}</option>`;
}

function auditChangeRows(diff) {
  if (!diff || !diff.changes || !diff.changes.length) {
    return `<tr><td colspan="7"><div class="empty">${icon('check')}<div><b>${esc(tr('Sin cambios entre snapshots'))}</b>
      <div class="tiny">${esc(tr('Las dos líneas base son equivalentes según los campos auditados.'))}</div></div></div></td></tr>`;
  }
  return diff.changes.map((c) => {
    const label = AUDIT_CHANGE_LABELS[c.type] || c.type;
    const evidence = (c.evidence || []).map((e) => `<div>· ${esc(e)}</div>`).join('');
    return `<tr>
      <td><span class="badge info">${esc(tr(label))}</span></td>
      <td class="tiny">${esc(c.entity)}</td>
      <td class="tiny">${esc(c.label || c.id)}</td>
      <td class="mono tiny">${esc(c.field || '—')}</td>
      <td class="mono tiny">${esc(compareValue(c.from))}</td>
      <td class="mono tiny">${esc(compareValue(c.to))}</td>
      <td class="tiny muted">${evidence || '—'}</td>
    </tr>`;
  }).join('');
}

async function renderAudits(withDiff = false) {
  setPageTitle('audits');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const list = await api('/audits?limit=50');
  const items = list.items || [];
  state.audits.items = items;
  if (items.length && !state.audits.to) state.audits.to = items[0].id;
  if (items.length > 1 && !state.audits.from) state.audits.from = items[1].id;

  let diff = state.audits.diff;
  if (withDiff && state.audits.from && state.audits.to && state.audits.from !== state.audits.to) {
    diff = await api(`/audits/${state.audits.from}/diff/${state.audits.to}`, { method: 'POST' });
    state.audits.diff = diff;
  }

  const optionsFrom = items.map((s) => auditSnapshotOption(s, state.audits.from)).join('');
  const optionsTo = items.map((s) => auditSnapshotOption(s, state.audits.to)).join('');
  const canCompare = items.length > 1;

  const rows = items.length ? items.map((s) => `<tr>
    <td><b>${esc(s.name || tr('Sin nombre'))}</b>${s.notes ? `<div class="tiny muted">${esc(s.notes)}</div>` : ''}</td>
    <td class="tiny">${esc(fmtDate(s.ts))}</td>
    <td class="mono tiny">${fmtBytes(s.raw_size || 0)} → ${fmtBytes(s.size || 0)}</td>
    <td class="tiny">${esc(s.site_id || 1)}</td>
    <td class="tiny">
      <button class="btn sm" data-action="audit-export" data-id="${s.id}" data-format="json">${esc(tr('JSON'))}</button>
      <button class="btn sm" data-action="audit-export" data-id="${s.id}" data-format="gzip">${esc(tr('GZIP'))}</button>
      ${canWrite() ? `<button class="btn sm danger" data-action="audit-delete" data-id="${s.id}">${esc(tr('Eliminar'))}</button>` : ''}
    </td>
  </tr>`).join('') : `<tr><td colspan="5"><div class="empty">${icon('shield')}<div><b>${esc(tr('Todavía no hay snapshots'))}</b>
      <div class="tiny">${esc(tr('Guarda una línea base, haz cambios en la red y comparámosla con otra snapshot.'))}</div></div></div></td></tr>`;

  const diffCard = diff ? `<div class="card mb">
    <div class="card-head"><h3>${esc(tr('Diff de auditoría'))}</h3>
      <span class="tiny muted">${esc(diff.from_name || ('#' + diff.from_id))} → ${esc(diff.to_name || ('#' + diff.to_id))}
        · ${fmtNum(diff.summary && diff.summary.total, 0)} ${esc(tr('cambios'))}
        ${diff.truncated ? `<span class="badge warning">${esc(tr('truncado'))}</span>` : ''}
      </span>
    </div>
    ${(diff.limitations || []).length ? `<div class="notice warn mb">${icon('warn')}<div>
      <ul class="diag-limits">${diff.limitations.map((l) => `<li>${esc(l)}</li>`).join('')}</ul></div></div>` : ''}
    <div class="table-wrap"><table class="compare-changes">
      <thead><tr><th>${esc(tr('Tipo'))}</th><th>${esc(tr('Entidad'))}</th><th>${esc(tr('Objeto'))}</th>
        <th>${esc(tr('Campo'))}</th><th>${esc(tr('Antes'))}</th><th>${esc(tr('Ahora'))}</th><th>${esc(tr('Evidencia'))}</th></tr></thead>
      <tbody>${auditChangeRows(diff)}</tbody>
    </table></div>
  </div>` : '';

  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">${esc(tr('Guarda una línea base del estado actual, realiza cambios y compara dos snapshots para ver qué cambió.'))}</div>
      ${canWrite() ? `<div class="row-gap">
        <input id="audit-name" class="input sm" maxlength="120" placeholder="${esc(tr('Nombre del snapshot (opcional)'))}">
        <button class="btn primary sm" data-action="audit-save">${esc(tr('Guardar snapshot'))}</button>
      </div>` : ''}
    </div>
    <div class="card mb">
      <div class="card-head"><h3>${esc(tr('Comparar snapshots'))}</h3></div>
      <div class="card-body row-gap">
        <label class="tiny">${esc(tr('Desde'))}
          <select class="select sm" data-action="audit-from" ${canCompare ? '' : 'disabled'}>${optionsFrom}</select>
        </label>
        <label class="tiny">${esc(tr('Hasta'))}
          <select class="select sm" data-action="audit-to" ${canCompare ? '' : 'disabled'}>${optionsTo}</select>
        </label>
        <button class="btn primary sm" data-action="audit-diff" ${canCompare ? '' : 'disabled'}>${esc(tr('Ver diff'))}</button>
      </div>
    </div>
    ${diffCard}
    <div class="card">
      <div class="card-head"><h3>${esc(tr('Líneas base guardadas'))}</h3>
        <span class="tiny muted">${fmtNum(items.length, 0)} ${esc(tr('snapshots'))}</span>
      </div>
      <div class="table-wrap"><table>
        <thead><tr><th>${esc(tr('Nombre'))}</th><th>${esc(tr('Fecha'))}</th><th>${esc(tr('Tamaño'))}</th>
          <th>${esc(tr('Sede'))}</th><th>${esc(tr('Acciones'))}</th></tr></thead>
        <tbody>${rows}</tbody>
      </table></div>
    </div>
    <div class="card pad mt"><div class="tiny muted">${esc(tr('Los snapshots se comprimen y se guardan sin secretos: solo se conservan los nombres de las claves de configuración, nunca sus valores.'))}</div></div>`;
}

/* ------------------------------------------------------------------ */
/* F18 custom dashboards                                               */
/* ------------------------------------------------------------------ */

function widgetBody(type, data) {
  if (data && data.error) return `<div class="notice err">${esc(data.error)}</div>`;
  const findItems = (obj) => {
    if (!obj || typeof obj !== 'object') return null;
    if (Array.isArray(obj.items)) return obj.items;
    if (Array.isArray(obj.nodes)) return obj.nodes;
    for (const key of Object.keys(obj)) {
      if (Array.isArray(obj[key]) && obj[key].length && typeof obj[key][0] === 'object') return obj[key];
    }
    return null;
  };
  const items = findItems(data);
  if (items && items.length) {
    const rows = items.slice(0, 5).map((row) => {
      const values = ['name', 'hostname', 'ip', 'ssid', 'bssid', 'type', 'port', 'status', 'signal_dbm', 'state', 'cidr']
        .filter((k) => row[k] !== undefined && row[k] !== null && row[k] !== '')
        .slice(0, 3)
        .map((k) => `<span class="tiny mono">${esc(String(row[k]))}</span>`)
        .join(' · ');
      return `<div class="dash-row">${values || '<span class="tiny muted">—</span>'}</div>`;
    }).join('');
    return `<div class="dash-list">${rows}${items.length > 5 ? `<div class="tiny muted">+ ${items.length - 5} ${esc(tr('más'))}</div>` : ''}</div>`;
  }
  if (data && typeof data === 'object') {
    const rows = Object.keys(data).filter((k) => ['items', 'total', 'generated_at'].indexOf(k) < 0).slice(0, 6)
      .map((k) => `<div class="dash-row"><span class="tiny muted">${esc(k)}</span><b class="tiny mono">${esc(String(data[k]))}</b></div>`).join('');
    if (rows) return `<div class="dash-list">${rows}</div>`;
  }
  return `<div class="tiny muted">${esc(tr('Sin datos'))}</div>`;
}

async function renderDashboards() {
  setPageTitle('dashboards');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const [list, catalog] = await Promise.all([api('/dashboards'), api('/dashboards/catalog')]);
  const dashboards = list.items || [];
  const selected = dashboards.find((d) => String(d.id) === String(state.dashboardId)) || dashboards[0] || null;
  state.dashboardId = selected ? selected.id : null;

  const options = catalog.items || [];
  const selector = dashboards.length ? `<select class="select sm" data-action="dash-select">
      ${dashboards.map((d) => `<option value="${d.id}" ${d.id === selected.id ? 'selected' : ''}>${esc(d.name)}</option>`).join('')}
    </select>` : `<span class="muted tiny">${esc(tr('Sin dashboards'))}</span>`;

  let widgetsHtml = '';
  if (selected && selected.widgets && selected.widgets.length) {
    const responses = await Promise.all(selected.widgets.map((w) => {
      const source = String((w.config && w.config.source) || '').replace(/^\/api\/v1/, '');
      return api(source).catch((e) => ({ error: e.message }));
    }));
    widgetsHtml = selected.widgets.map((w, i) => {
      const title = (w.config && w.config.title) || w.type;
      return `<div class="card dash-widget" draggable="true" data-widget-id="${esc(w.id)}"
          style="grid-column:span ${Math.min(12, Math.max(2, w.size && w.size.w ? w.size.w : 4))}">
        <div class="card-head"><h3>${esc(tr(title))}</h3>
          <button class="btn icon sm" title="${esc(tr('Quitar widget'))}" data-action="dash-remove-widget" data-id="${esc(w.id)}">×</button>
        </div>
        <div class="dash-widget-body">${widgetBody(w.type, responses[i])}</div>
      </div>`;
    }).join('');
  }

  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">${esc(tr('Crea vistas con hasta 16 widgets. Arrastra una tarjeta para reordenarla; la rejilla tiene 12 columnas.'))}</div>
      <div class="row-gap">
        ${selector}
        <button class="btn sm" data-action="dash-example">${icon('dashboard')} ${esc(tr('Ejemplo "Mi red"'))}</button>
      </div>
    </div>
    <div class="card pad mb">
      <div class="spread">
        <div class="field" style="flex:1"><label for="dash-name">${esc(tr('Nombre del dashboard'))}</label>
          <input id="dash-name" class="input" maxlength="80" placeholder="${esc(tr('Mi red'))}"></div>
        <button class="btn primary" data-action="dash-create">${icon('check')} ${esc(tr('Crear'))}</button>
        ${selected ? `<button class="btn danger" data-action="dash-delete" data-id="${selected.id}">${esc(tr('Eliminar'))}</button>` : ''}
      </div>
    </div>
    ${selected ? `
    <div class="card pad mb">
      <div class="spread"><b class="small">${esc(tr('Añadir widget'))}</b>
        <div class="row-gap"><select id="dash-widget-type" class="select sm">
          ${options.map((o) => `<option value="${esc(o.type)}">${esc(tr(o.label))}</option>`).join('')}
        </select><button class="btn sm" data-action="dash-add-widget">${icon('plus')} ${esc(tr('Añadir'))}</button></div>
      </div>
    </div>
    <div class="dash-grid mb">${widgetsHtml || `<div class="empty">${icon('dashboard')}<div><b>${esc(tr('Dashboard vacío'))}</b>
      <div class="tiny">${esc(tr('Añade widgets para construir tu vista.'))}</div></div></div>`}</div>` : `
    <div class="card pad"><div class="empty">${icon('dashboard')}<div><b>${esc(tr('Todavía no hay dashboards'))}</b>
      <div class="tiny">${esc(tr('Pulsa en ejemplo "Mi red" para crear uno con cuatro widgets.'))}</div></div></div></div>`}`;

  content.querySelectorAll('.dash-widget').forEach((card) => {
    card.addEventListener('dragstart', (e) => {
      e.dataTransfer.setData('text/plain', card.dataset.widgetId);
      e.dataTransfer.effectAllowed = 'move';
      card.classList.add('dragging');
    });
    card.addEventListener('dragend', () => card.classList.remove('dragging'));
    card.addEventListener('dragover', (e) => { e.preventDefault(); card.classList.add('drag-over'); });
    card.addEventListener('dragleave', () => card.classList.remove('drag-over'));
    card.addEventListener('drop', async (e) => {
      e.preventDefault();
      card.classList.remove('drag-over');
      const from = e.dataTransfer.getData('text/plain');
      const to = card.dataset.widgetId;
      if (!from || from === to || !selected) return;
      const widgets = selected.widgets.map((w) => Object.assign({}, w));
      const i = widgets.findIndex((w) => w.id === from);
      const j = widgets.findIndex((w) => w.id === to);
      if (i < 0 || j < 0) return;
      const [moved] = widgets.splice(i, 1);
      widgets.splice(j, 0, moved);
      widgets.forEach((w, idx) => { w.position = { x: (idx % 2) * 6, y: Math.floor(idx / 2) }; });
      try {
        await api('/dashboards/' + selected.id, { method: 'PUT', body: { name: selected.name, site_id: selected.site_id, widgets } });
        await renderDashboards();
      } catch (err) { toast(err.message, { type: 'err' }); }
    });
  });
}

/* ------------------------------------------------------------------ */
/* F11 WiFi analysis                                                   */
/* ------------------------------------------------------------------ */

function wifiSignalCell(dbm) {
  const v = Number(dbm) || 0;
  const tone = v >= -50 ? 'var(--green)' : v >= -70 ? 'var(--amber)' : 'var(--red)';
  return `<span class="mono" style="color:${tone}">${esc(String(v))} dBm</span>`;
}

async function renderWiFi() {
  setPageTitle('wifi');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const [status, apData, clientData, channelData] = await Promise.all([
    api('/wifi/status'), api('/wifi/aps'), api('/wifi/clients'), api('/wifi/channels')
  ]);
  const aps = apData.items || [];
  const clients = clientData.items || [];
  const channels = channelData.items || [];
  const rogue = aps.filter((ap) => ap.rogue).length;
  const busy = channels.filter((ch) => ch.load === 'alta').length;
  const canCapture = !!status.capture_available;

  const apRows = aps.length ? aps.map((ap) => `
    <tr>
      <td><div class="name-cell"><b>${esc(ap.ssid || tr('(oculta)'))}</b><span class="mono-ip">${esc(ap.bssid)}</span></div></td>
      <td>${esc(ap.channel || '—')} <span class="tiny muted">${esc(ap.band || '')}</span></td>
      <td><span class="badge ${ap.encryption === 'abierta' ? 'critical' : 'info'}">${esc(ap.encryption)}</span></td>
      <td>${wifiSignalCell(ap.signal_dbm)}</td>
      <td>${fmtNum(ap.clients || 0)}</td>
      <td>${ap.rogue
        ? `<span class="badge critical">${esc(tr('No autorizado'))}</span>
           <details class="diag-details"><summary class="tiny">${esc(tr('Evidencia'))}</summary>
             <div class="diag-evidence"><div>· ${esc(ap.rogue_info)}</div>
             <div>· ${esc(tr('Confianza'))}: ${ap.rogue_confidence || 0}%</div></div></details>`
        : `<span class="badge up">${esc(tr('Autorizado'))}</span>`}</td>
      <td class="tiny muted nowrap">${fmtDate(ap.last_seen)}</td>
    </tr>`).join('') : `<tr><td colspan="7"><div class="empty">${icon('wifi')}<div><b>${esc(tr('Sin APs observados'))}</b>
      <div class="tiny">${esc(tr('Captura beacons con una interfaz en modo monitor para poblar esta vista.'))}</div></div></div></td></tr>`;

  const channelRows = channels.length ? channels.map((ch) => `
    <tr>
      <td><b>${esc(ch.channel)}</b> <span class="tiny muted">${esc(ch.band)}</span></td>
      <td>${fmtNum(ch.aps)}</td><td>${fmtNum(ch.clients)}</td><td>${fmtNum(ch.overlap)}</td>
      <td><span class="badge ${ch.load === 'alta' ? 'critical' : ch.load === 'media' ? 'warning' : 'up'}">${esc(tr(ch.load))}</span></td>
      <td class="tiny">${esc(ch.suggest || '—')}</td>
    </tr>`).join('') : `<tr><td colspan="6"><div class="empty">${icon('radar')}<div><b>${esc(tr('Sin datos de canales'))}</b></div></div></td></tr>`;

  const clientRows = clients.length ? clients.slice(0, 200).map((c) => `
    <tr><td class="mono-ip">${esc(c.mac)}</td><td class="mono-ip">${esc(c.ap_bssid || '—')}</td>
      <td>${wifiSignalCell(c.signal_dbm)}</td><td class="tiny muted">${fmtDate(c.last_seen)}</td></tr>`).join('')
    : `<tr><td colspan="4"><div class="empty">${icon('network')}<div><b>${esc(tr('Sin clientes observados'))}</b></div></div></td></tr>`;

  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">${esc(tr('Análisis pasivo de 802.11: APs, clientes, interferencia por canal y APs no autorizados según el inventario de referencia.'))}</div>
      <button class="btn sm" data-action="wifi-scan">${icon('radar')} ${esc(tr('Capturar ahora'))}</button>
    </div>
    <div class="grid cols-4 mb">
      ${statCard({ label: tr('APs'), value: fmtNum(aps.length), hint: tr('observaciones persistentes'), icon: 'wifi', tone: 'violet' })}
      ${statCard({ label: tr('Clientes'), value: fmtNum(clients.length), hint: tr('estaciones detectadas'), icon: 'network', tone: 'accent' })}
      ${statCard({ label: tr('APs no autorizados'), value: fmtNum(rogue), hint: tr('según inventario configurado'), icon: 'shield', tone: 'red' })}
      ${statCard({ label: tr('Canales saturados'), value: fmtNum(busy), hint: tr('carga alta'), icon: 'radar', tone: 'amber' })}
    </div>
    <div class="card mb">
      <div class="card-head"><h3>${esc(tr('Puntos de acceso'))}</h3><span class="tiny muted">${esc(status.interface || tr('sin interfaz configurada'))} · ${esc(tr('ventana'))}: ${esc(String(status.scan_seconds || 10))}s</span></div>
      <div class="table-wrap"><table>
        <thead><tr><th>${esc(tr('SSID / BSSID'))}</th><th>${esc(tr('Canal'))}</th><th>${esc(tr('Cifrado'))}</th>
          <th>${esc(tr('Señal'))}</th><th>${esc(tr('Clientes'))}</th><th>${esc(tr('Estado'))}</th><th>${esc(tr('Visto'))}</th></tr></thead>
        <tbody>${apRows}</tbody>
      </table></div>
    </div>
    <div class="grid cols-2 mb">
      <div class="card">
        <div class="card-head"><h3>${esc(tr('Canales e interferencias'))}</h3><span class="badge info">${esc(tr('Heurístico'))}</span></div>
        <div class="table-wrap"><table>
          <thead><tr><th>${esc(tr('Canal'))}</th><th>${esc(tr('APs'))}</th><th>${esc(tr('Clientes'))}</th>
            <th>${esc(tr('Solape'))}</th><th>${esc(tr('Carga'))}</th><th>${esc(tr('Sugerencia'))}</th></tr></thead>
          <tbody>${channelRows}</tbody>
        </table></div>
      </div>
      <div class="card">
        <div class="card-head"><h3>${esc(tr('Clientes por AP'))}</h3><span class="tiny muted">${esc(tr('últimas observaciones'))}</span></div>
        <div class="table-wrap"><table>
          <thead><tr><th>${esc(tr('MAC'))}</th><th>${esc(tr('AP'))}</th><th>${esc(tr('Señal'))}</th><th>${esc(tr('Visto'))}</th></tr></thead>
          <tbody>${clientRows}</tbody>
        </table></div>
      </div>
    </div>
    <div class="card pad">
      <div class="spread mb"><h3 style="margin:0;font-size:13.5px">${esc(tr('Limitaciones'))}</h3><span class="badge ${canCapture ? 'up' : 'warning'}">${esc(canCapture ? tr('Captura disponible') : tr('Captura no compilada'))}</span></div>
      <ul class="diag-limits">
        <li>${esc(tr('La interfaz debe configurarse previamente en modo monitor; NetDiag no cambia el modo del dispositivo.'))}</li>
        <li>${esc(tr('El binario portable no incluye captura 802.11; se requiere compilar con -tags pcap.'))}</li>
        <li>${esc(tr('El SSID puede suplantarse: el BSSID autorizado es la evidencia más fuerte; los SSID conocidos tienen menor confianza.'))}</li>
        <li>${esc(tr('El solape en 2,4 GHz agrupa canales con separación inferior a cinco; en 5/6 GHz solo se considera el mismo canal.'))}</li>
      </ul>
    </div>`;
}

const VPN_TYPE_LABELS = { wireguard: 'WireGuard', openvpn: 'OpenVPN', ipsec: 'IPsec/IKE', pptp: 'PPTP' };

async function renderVPN() {
  setPageTitle('vpn');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const data = await api('/vpn/tunnels');
  const items = data.items || [];
  const cfg = data.config || {};
  const active = items.filter((t) => t.state === 'active').length;

  const rows = items.length ? items.map((t) => `
    <tr>
      <td><b>${esc(tr(VPN_TYPE_LABELS[t.type] || t.type))}</b></td>
      <td>${t.local_ip ? `<span class="mono-ip">${esc(t.local_ip)}</span>${t.local_port ? `:${t.local_port}` : ''}` : '<span class="muted">—</span>'}</td>
      <td>${t.remote_ip ? `<span class="mono-ip">${esc(t.remote_ip)}</span>${t.remote_port ? `:${t.remote_port}` : ''}` : '<span class="muted">—</span>'}</td>
      <td><span class="badge ${t.state === 'active' ? 'ok' : t.state === 'stale' ? 'muted' : 'info'}">${esc(tr(t.state === 'active' ? 'Activo' : t.state === 'stale' ? 'Obsoleto' : 'Observado'))}</span></td>
      <td><div class="tiny mono">${fmtBytes(t.bytes_recv)} ↓ / ${fmtBytes(t.bytes_sent)} ↑</div></td>
      <td>${diagConfidenceBar(t.confidence)}</td>
      <td class="right nowrap"><details class="diag-details"><summary>${esc(tr('Evidencia'))}</summary>
        <div class="diag-evidence">${(t.evidence || []).map((e) => `<div>· ${esc(e)}</div>`).join('')}</div>
      </details></td>
    </tr>`).join('') : `<tr><td colspan="7"><div class="empty">${icon('network')}<div><b>${esc(tr('Sin túneles detectados'))}</b>
      <div class="tiny">${esc(tr('No se encontraron indicios de VPN en las tablas locales ni en los puertos conocidos.'))}</div></div></div></td></tr>`;

  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">${esc(tr('Detección pasiva y heurística a partir de tablas del kernel (sockets e interfaces) y puertos conocidos. No se inspecciona ni descifra tráfico.'))}</div>
      <button class="btn sm" data-action="vpn-scan">${icon('radar')} ${esc(tr('Observar ahora'))}</button>
    </div>
    <div class="grid cols-4 mb">
      ${statCard({ label: tr('Túneles'), value: fmtNum(items.length), hint: tr('observaciones persistentes'), icon: 'network', tone: 'violet' })}
      ${statCard({ label: tr('Activos'), value: fmtNum(active), hint: tr('con endpoints o tráfico'), icon: 'check', tone: 'green' })}
      ${statCard({ label: tr('Confianza mínima'), value: String(cfg.min_confidence !== undefined ? cfg.min_confidence : 60), hint: tr('umbral configurado'), icon: 'shield', tone: 'accent' })}
      ${statCard({ label: tr('Intervalo'), value: esc(cfg.interval || '5m0s'), hint: tr('observación periódica'), icon: 'clock', tone: 'amber' })}
    </div>
    <div class="card mb">
      <div class="card-head"><h3>${esc(tr('Túneles observados'))}</h3><span class="tiny muted">${esc(tr('Actualizado en cada observación'))}</span></div>
      <div class="table-wrap"><table>
        <thead><tr><th>${esc(tr('Tipo'))}</th><th>${esc(tr('Local'))}</th><th>${esc(tr('Remoto'))}</th><th>${esc(tr('Estado'))}</th><th>${esc(tr('Tráfico'))}</th><th>${esc(tr('Confianza'))}</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
      </table></div>
    </div>
    <div class="card pad">
      <div class="spread mb"><h3 style="margin:0;font-size:13.5px">${esc(tr('Limitaciones'))}</h3><span class="badge info">${esc(tr('Heurístico'))}</span></div>
      <ul class="diag-limits">
        <li>${esc(tr('Los contadores de interfaz son acumulativos desde el arranque, no tasas instantáneas.'))}</li>
        <li>${esc(tr('TCP/443 es ambiguo: requiere otra evidencia para confirmar OpenVPN.'))}</li>
        <li>${esc(tr('Un túnel inactivo o cifrado puede no dejar rastros en las tablas consultadas.'))}</li>
      </ul>
    </div>`;
}

/* ------------------------------------------------------------------ */
/* Alerts                                                              */
/* ------------------------------------------------------------------ */

const ALERT_TAB_TITLES = { alertas: 'Alertas', reglas: 'Reglas', canales: 'Canales' };

async function renderAlerts(tab = 'alertas') {
  state.alertsTab = tab;
  setPageTitle('alerts', ALERT_TAB_TITLES[tab] || 'Alertas');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const body = tab === 'reglas'
    ? await alertsRulesBody()
    : tab === 'canales'
      ? await alertsChannelsBody()
      : await alertsListBody();
  content.innerHTML = `
    <div class="tabs" style="margin-top:0">
      ${[['alertas', 'Alertas'], ['reglas', 'Reglas'], ['canales', 'Canales']].map(([key, label]) =>
        `<div class="tab ${tab === key ? 'active' : ''}" data-action="alerts-tab" data-tab="${key}">${label}</div>`).join('')}
    </div>
    ${body}`;
}

async function alertsListBody() {
  const f = state.alerts;
  const q = new URLSearchParams();
  q.set('page', f.page); q.set('limit', f.limit);
  if (f.status) q.set('status', f.status);
  if (f.severity) q.set('severity', f.severity);
  if (f.search) q.set('search', f.search);
  const [res, stats, rulesRes] = await Promise.all([api('/alerts?' + q.toString()), api('/alerts/stats'), api('/alerts/rules')]);
  const rulesCount = (rulesRes.items || []).length;
  const items = res.items || [];
  const pages = Math.max(1, Math.ceil((res.total || 0) / f.limit));
  const rows = items.length ? items.map((a) => {
    const actions = [];
    if (a.status === 'active') actions.push(`<button class="btn sm" data-action="alert-ack" data-id="${a.id}">Reconocer</button>`);
    if (a.status !== 'resolved') actions.push(`<button class="btn sm" data-action="alert-resolve" data-id="${a.id}">Resolver</button>`);
    return `<tr>
      <td><span class="sev-dot sev-${esc(a.severity)}" title="${esc(SEV_LABELS[a.severity] || a.severity)}"></span></td>
      <td><div class="name-cell"><b>${esc(a.title)}</b><span class="ellipsis" style="display:block">${esc(a.message)}</span></div></td>
      <td>${a.host_id ? `<a href="#/hosts/${a.host_id}" class="mono-ip">${esc(a.host_name || a.host_ip || '#' + a.host_id)}</a>${a.host_name && a.host_ip ? `<div class="tiny muted mono">${esc(a.host_ip)}</div>` : ''}` : '<span class="muted">Global</span>'}</td>
      <td class="tiny muted">${esc(a.rule_name || '—')}</td>
      <td>${sevBadge(a.severity)}</td>
      <td><span class="badge ${a.status === 'active' ? 'down' : a.status === 'acked' ? 'warning' : 'up'}">${a.status === 'active' ? 'Activa' : a.status === 'acked' ? 'Reconocida' : 'Resuelta'}</span></td>
      <td class="tiny muted nowrap">${ago(a.created_at)}</td>
      <td class="right nowrap">${actions.join(' ')}</td>
    </tr>`;
  }).join('') : `<tr><td colspan="8"><div class="empty">${icon('alerts')}Sin alertas para estos filtros</div></td></tr>`;

  return `
    <div class="grid cols-4 mb">
      <div class="card stat red"><div class="label">Activas</div><div class="value">${fmtNum(stats.active)}</div><div class="hint">${fmtNum(stats.critical)} críticas · ${fmtNum(stats.warning)} avisos</div></div>
      <div class="card stat amber"><div class="label">Reconocidas</div><div class="value">${fmtNum(stats.acked)}</div><div class="hint">pendientes de resolver</div></div>
      <div class="card stat green"><div class="label">Resueltas</div><div class="value">${fmtNum(stats.resolved)}</div><div class="hint">histórico total</div></div>
      <div class="card stat accent"><div class="label">Reglas</div><div class="value">${fmtNum(rulesCount)}</div><div class="hint">configuración de umbrales</div></div>
    </div>
    <div class="card">
      <div class="filters">
        <div class="search">${icon('search')}
          <input class="input" data-alerts-search placeholder="Buscar título, mensaje o IP…" value="${esc(f.search)}">
        </div>
        <select class="select" data-alerts-filter="status">
          ${[['active', 'Activas'], ['acked', 'Reconocidas'], ['resolved', 'Resueltas'], ['all', 'Todas']].map(([v, l]) =>
            `<option value="${v}" ${f.status === v ? 'selected' : ''}>${l}</option>`).join('')}
        </select>
        <select class="select" data-alerts-filter="severity">
          ${[['', 'Toda severidad'], ['critical', 'Crítica'], ['warning', 'Aviso'], ['info', 'Info']].map(([v, l]) =>
            `<option value="${v}" ${f.severity === v ? 'selected' : ''}>${l}</option>`).join('')}
        </select>
        <div class="right row">
          <button class="btn sm" data-action="reload">${icon('refresh')} Actualizar</button>
          <a class="btn sm" href="#/alerts/reglas">${icon('settings')} Reglas</a>
        </div>
      </div>
      <div class="table-wrap"><table>
        <thead><tr><th></th><th>Alerta</th><th>Dispositivo</th><th>Regla</th><th>Severidad</th><th>Estado</th><th>Creada</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
      </table></div>
      <div class="pagination">
        <span>${fmtNum(res.total)} alertas · página ${f.page} de ${pages}</span>
        <div class="spacer right"></div>
        <button class="btn sm" data-action="alerts-page" data-page="${f.page - 1}" ${f.page <= 1 ? 'disabled' : ''}>Anterior</button>
        <button class="btn sm" data-action="alerts-page" data-page="${f.page + 1}" ${f.page >= pages ? 'disabled' : ''}>Siguiente</button>
      </div>
    </div>`;
}

function ruleConfigSummary(rule) {
  const cfg = rule.config || {};
  switch (rule.type) {
    case 'host_down': return cfg.down_after_sec ? `tras ${cfg.down_after_sec}s sin respuesta` : '';
    case 'latency_high': return cfg.threshold_ms ? `> ${cfg.threshold_ms} ms (${cfg.consecutive || 1} muestras)` : '';
    case 'port_opened': return cfg.ignore_ephemeral ? 'ignora puertos efímeros' : '';
    case 'subnet_usage': return cfg.percent ? `> ${cfg.percent}% de ocupación` : '';
    default: return '';
  }
}

async function alertsRulesBody() {
  const [ruleRes, chanRes] = await Promise.all([api('/alerts/rules'), api('/alerts/channels')]);
  state.rules = ruleRes.items || [];
  state.channels = chanRes.items || [];
  const channelName = (id) => (state.channels.find((c) => c.id === id) || {}).name || `#${id}`;
  const rows = state.rules.length ? state.rules.map((r) => `
    <tr>
      <td><span class="toggle ${r.enabled ? 'on' : ''}" data-action="rule-toggle" data-id="${r.id}" data-enabled="${r.enabled}" title="${r.enabled ? 'Deshabilitar' : 'Habilitar'}" style="cursor:pointer"></span></td>
      <td><div class="name-cell"><b>${esc(r.name)}</b><span>${esc(RULE_LABELS[r.type] || r.type)}</span></div></td>
      <td class="tiny muted">${esc(ruleConfigSummary(r)) || '—'}</td>
      <td>${sevBadge(r.severity)}</td>
      <td class="mono tiny">${r.cooldown_sec >= 60 ? Math.round(r.cooldown_sec / 60) + ' min' : (r.cooldown_sec || 0) + ' s'}</td>
      <td class="tiny muted">${(r.channel_ids || []).length ? r.channel_ids.map((id) => `<span class="tag">${esc(channelName(id))}</span>`).join('') : '<span class="muted">Todos</span>'}</td>
      <td>${r.builtin ? '<span class="badge info">Integrada</span>' : '<span class="badge unknown">Personal</span>'}</td>
      <td class="right nowrap">
        <button class="btn sm" data-action="rule-edit" data-id="${r.id}">${icon('edit')} Editar</button>
        ${r.builtin ? '' : `<button class="btn sm danger" data-action="rule-delete" data-id="${r.id}" title="Eliminar">${icon('trash')}</button>`}
      </td>
    </tr>`).join('') : `<tr><td colspan="8"><div class="empty">${icon('alerts')}Sin reglas</div></td></tr>`;
  return `
    <div class="spread mb">
      <div class="muted small">${state.rules.length} reglas · se evalúan después de cada escaneo, con cooldown y deduplicación.</div>
      <button class="btn primary" data-action="rule-new">${icon('plus')} Nueva regla</button>
    </div>
    <div class="card"><div class="table-wrap"><table>
      <thead><tr><th>Activa</th><th>Regla</th><th>Condición</th><th>Severidad</th><th>Cooldown</th><th>Canales</th><th>Origen</th><th></th></tr></thead>
      <tbody>${rows}</tbody>
    </table></div></div>`;
}

async function alertsChannelsBody() {
  if (!state.channels.length) {
    const res = await api('/alerts/channels');
    state.channels = res.items || [];
    state.channelMeta = { types: res.types || [], secret_fields: res.secret_fields || {} };
  }
  const rows = state.channels.length ? state.channels.map((c) => `
    <tr>
      <td><span class="toggle ${c.enabled ? 'on' : ''}" data-action="channel-toggle" data-id="${c.id}" data-enabled="${c.enabled}" style="cursor:pointer" title="${c.enabled ? 'Deshabilitar' : 'Habilitar'}"></span></td>
      <td><div class="name-cell"><b>${esc(c.name)}</b><span>${esc(CHANNEL_LABELS[c.type] || c.type)}</span></div></td>
      <td>${c.has_secret ? '<span class="badge up">Credencial cifrada</span>' : '<span class="badge unknown">Sin secreto</span>'}</td>
      <td class="tiny muted">${c.last_test_at ? fmtDate(c.last_test_at) : 'Nunca'}</td>
      <td class="tiny" style="color:var(--red)">${esc(c.last_error || '')}</td>
      <td class="right nowrap">
        <button class="btn sm" data-action="channel-test" data-id="${c.id}">${icon('test')} Probar</button>
        <button class="btn sm" data-action="channel-edit" data-id="${c.id}">${icon('edit')} Editar</button>
        <button class="btn sm danger" data-action="channel-delete" data-id="${c.id}">${icon('trash')}</button>
      </td>
    </tr>`).join('') : `<tr><td colspan="6"><div class="empty">${icon('alerts')}Sin canales configurados</div></td></tr>`;
  return `
    <div class="spread mb">
      <div class="muted small">Los destinos se guardan cifrados con AES-256-GCM. Las credenciales nunca se muestran después de guardarlas.</div>
      <button class="btn primary" data-action="channel-new">${icon('plus')} Nuevo canal</button>
    </div>
    <div class="card"><div class="table-wrap"><table>
      <thead><tr><th>Activo</th><th>Canal</th><th>Credencial</th><th>Última prueba</th><th>Último error</th><th></th></tr></thead>
      <tbody>${rows}</tbody>
    </table></div></div>`;
}

/* ---------------- rule modal ---------------- */

const RULE_TYPE_OPTIONS = [
  ['host_down', 'Host caído'],
  ['host_new', 'Host nuevo'],
  ['latency_high', 'Latencia alta'],
  ['port_opened', 'Puerto abierto'],
  ['port_closed', 'Puerto cerrado'],
  ['service_changed', 'Servicio cambiado'],
  ['device_changed', 'Dispositivo cambiado'],
  ['subnet_usage', 'Uso de subred'],
  ['scan_failed', 'Escaneo fallido'],
];

function ruleConfigFieldsHTML(type, cfg) {
  cfg = cfg || {};
  const num = (key, label, value, help) => `
    <div class="field"><label>${esc(label)}</label>
      <input class="input" name="cfg_${key}" type="number" min="0" step="any" value="${value === undefined || value === null ? '' : esc(value)}">
      ${help ? `<span class="help">${esc(help)}</span>` : ''}
    </div>`;
  switch (type) {
    case 'host_down':
      return num('down_after_sec', 'Segundos caído antes de alertar', cfg.down_after_sec !== undefined ? cfg.down_after_sec : 120, 'Evita falsos positivos por un fallo puntual.');
    case 'latency_high':
      return `<div class="form-grid">
        ${num('threshold_ms', 'Umbral de latencia (ms)', cfg.threshold_ms !== undefined ? cfg.threshold_ms : 150, 'RTT medio por encima de este valor.')}
        ${num('consecutive', 'Muestras consecutivas', cfg.consecutive !== undefined ? cfg.consecutive : 3, 'Número de escaneos seguidos.')}
      </div>`;
    case 'port_opened':
      return `<label class="check"><input type="checkbox" name="cfg_ignore_ephemeral" ${cfg.ignore_ephemeral !== false ? 'checked' : ''}> Ignorar puertos efímeros (>= 32768)</label>`;
    case 'subnet_usage':
      return num('percent', 'Porcentaje de ocupación', cfg.percent !== undefined ? cfg.percent : 90, 'Alerta cuando la subred supera este uso.');
    default:
      return `<div class="notice"><span>Esta regla no necesita parámetros adicionales.</span></div>`;
  }
}

function openRuleModal(id) {
  const rule = id ? state.rules.find((r) => r.id === id) : null;
  const channels = state.channels || [];
  const selected = new Set(rule ? (rule.channel_ids || []) : []);
  const type = rule ? rule.type : 'host_down';
  state.ruleEditingConfig = rule ? Object.assign({}, rule.config || {}) : {};
  const body = `
    <form data-form="rule" data-id="${rule ? rule.id : ''}">
      <input type="hidden" name="builtin" value="${rule && rule.builtin ? '1' : '0'}">
      <div class="form-grid">
        <div class="field full"><label>Nombre</label>
          <input class="input" name="name" required maxlength="80" value="${esc(rule ? rule.name : '')}" placeholder="Ej.: Latencia alta en oficina"></div>
        <div class="field"><label>Tipo de regla</label>
          <select class="select" name="type" id="rule-type">
            ${RULE_TYPE_OPTIONS.map(([v, l]) => `<option value="${v}" ${type === v ? 'selected' : ''}>${l}</option>`).join('')}
          </select></div>
        <div class="field"><label>Severidad</label>
          <select class="select" name="severity">
            ${[['info', 'Info'], ['warning', 'Aviso'], ['critical', 'Crítica']].map(([v, l]) =>
              `<option value="${v}" ${rule && rule.severity === v ? 'selected' : ''}>${l}</option>`).join('')}
          </select></div>
        <div class="field"><label>Cooldown (segundos)</label>
          <input class="input" name="cooldown_sec" type="number" min="0" max="604800" value="${rule ? rule.cooldown_sec : 900}">
          <span class="help">Tiempo mínimo entre alertas idénticas.</span></div>
        <div class="field"><label>Estado</label>
          <label class="check"><input type="checkbox" name="enabled" ${!rule || rule.enabled ? 'checked' : ''}> Regla activa</label></div>
      </div>
      <div class="divider"></div>
      <div id="rule-config">${ruleConfigFieldsHTML(type, rule ? rule.config : {})}</div>
      <div class="divider"></div>
      <div class="field"><label>Canales de notificación</label>
        ${channels.length ? `<div class="row wrap">${channels.map((c) => `
          <label class="check"><input type="checkbox" name="channel" value="${c.id}" ${selected.has(c.id) ? 'checked' : ''}> ${esc(c.name)}</label>`).join('')}</div>
          <span class="help">Sin selección: todos los canales habilitados.</span>`
        : '<span class="help">No hay canales creados; las alertas solo aparecerán en el panel.</span>'}
      </div>
    </form>`;
  openModal(rule ? 'Editar regla' : 'Nueva regla', body, {
    wide: true,
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="rule-save" data-id="${rule ? rule.id : ''}">${icon('check')} Guardar</button>`,
  });
  const sel = document.getElementById('rule-type');
  if (sel) {
    sel.addEventListener('change', () => {
      const box = document.getElementById('rule-config');
      if (box) box.innerHTML = ruleConfigFieldsHTML(sel.value, {});
    });
  }
}

async function submitRule(form) {
  const fd = new FormData(form);
  const type = String(fd.get('type') || '');
  const cfg = Object.assign({}, state.ruleEditingConfig || {});
  const numeric = ['down_after_sec', 'threshold_ms', 'consecutive', 'percent'];
  numeric.forEach((key) => {
    const raw = fd.get('cfg_' + key);
    if (raw !== null && String(raw).trim() !== '') cfg[key] = Number(raw);
    else delete cfg[key];
  });
  if (type === 'port_opened') cfg.ignore_ephemeral = fd.get('cfg_ignore_ephemeral') === 'on';
  const channelIDs = fd.getAll('channel').map(Number).filter((n) => n > 0);
  const id = Number(form.dataset.id || 0);
  const payload = {
    name: String(fd.get('name') || '').trim(),
    type,
    enabled: fd.get('enabled') === 'on',
    severity: String(fd.get('severity') || 'warning'),
    cooldown_sec: Number(fd.get('cooldown_sec') || 0),
    channel_ids: channelIDs,
    config: cfg,
  };
  if (!payload.name) throw new Error('El nombre es obligatorio');
  if (id) await api('/alerts/rules/' + id, { method: 'PUT', body: payload });
  else await api('/alerts/rules', { method: 'POST', body: payload });
  closeModal();
  toast(id ? 'Regla actualizada' : 'Regla creada', { type: 'ok' });
  await renderAlerts('reglas');
}

async function deleteRule(id) {
  if (!confirm('¿Eliminar esta regla personalizada?')) return;
  await api('/alerts/rules/' + id, { method: 'DELETE' });
  toast('Regla eliminada', { type: 'ok' });
  await renderAlerts('reglas');
}

async function toggleRule(id, enabled) {
  const rule = state.rules.find((r) => r.id === id);
  if (!rule) return;
  const payload = {
    name: rule.name, type: rule.type, enabled, severity: rule.severity,
    cooldown_sec: rule.cooldown_sec, channel_ids: rule.channel_ids || [], config: rule.config || {},
  };
  await api('/alerts/rules/' + id, { method: 'PUT', body: payload });
  toast(enabled ? 'Regla habilitada' : 'Regla deshabilitada', { type: 'ok' });
  await renderAlerts('reglas');
}

/* ---------------- channel modal ---------------- */

const CHANNEL_FIELDS = {
  webhook: [
    ['url', 'URL del webhook', 'https://example.com/hook', 'text'],
    ['secret', 'Secreto HMAC (opcional)', 'dejar en blanco para conservar', 'secret'],
  ],
  slack: [['url', 'Webhook de Slack', 'https://hooks.slack.com/services/…', 'secret']],
  discord: [['url', 'Webhook de Discord', 'https://discord.com/api/webhooks/…', 'secret']],
  telegram: [
    ['token', 'Token del bot', '123456:ABC…', 'secret'],
    ['chat_id', 'Chat ID', '-1001234567890', 'text'],
  ],
  pagerduty: [['routing_key', 'Routing key', 'Clave de integración', 'secret']],
  smtp: [
    ['host', 'Servidor SMTP', 'smtp.example.com', 'text'],
    ['port', 'Puerto', '587', 'number'],
    ['from', 'Remitente', 'netdiag@example.com', 'text'],
    ['to', 'Destinatarios (separados por comas)', 'noc@example.com', 'text'],
    ['username', 'Usuario', 'usuario', 'text'],
    ['password', 'Contraseña', 'dejar en blanco para conservar', 'secret'],
    ['tls', 'TLS implícito (SMTPS)', '', 'bool'],
    ['starttls', 'STARTTLS', '', 'bool'],
  ],
  syslog: [
    ['addr', 'Dirección', '192.168.1.10:514', 'text'],
    ['network', 'Transporte', 'udp', 'select:udp,tcp'],
  ],
};

function channelConfigFieldsHTML(type, cfg) {
  cfg = cfg || {};
  const fields = CHANNEL_FIELDS[type] || [];
  return fields.map(([key, label, placeholder, kind]) => {
    const value = cfg[key];
    if (kind === 'bool') {
      return `<label class="check"><input type="checkbox" name="ch_${key}" ${value ? 'checked' : ''}> ${esc(label)}</label>`;
    }
    if (kind.startsWith('select:')) {
      const options = kind.slice(7).split(',');
      return `<div class="field"><label>${esc(label)}</label>
        <select class="select" name="ch_${key}">${options.map((o) => `<option value="${o}" ${String(value) === o ? 'selected' : ''}>${o.toUpperCase()}</option>`).join('')}</select></div>`;
    }
    if (kind === 'secret') {
      return `<div class="field full"><label>${esc(label)}</label>
        <input class="input" name="ch_${key}" autocomplete="new-password" value="${esc(value === '********' ? '' : value || '')}" placeholder="${esc(placeholder)}">
        <span class="help">Se cifra en la base de datos; déjalo en blanco para conservar el valor actual.</span></div>`;
    }
    return `<div class="field"><label>${esc(label)}</label>
      <input class="input" name="ch_${key}" ${kind === 'number' ? 'type="number"' : ''} value="${esc(value === undefined || value === null ? '' : value)}" placeholder="${esc(placeholder)}"></div>`;
  }).join('');
}

function openChannelModal(id) {
  const ch = id ? state.channels.find((c) => c.id === id) : null;
  const type = ch ? ch.type : 'webhook';
  state.channelEditingType = type;
  const body = `
    <form data-form="channel" data-id="${ch ? ch.id : ''}">
      <div class="form-grid">
        <div class="field"><label>Nombre</label>
          <input class="input" name="name" required maxlength="80" value="${esc(ch ? ch.name : '')}" placeholder="Ej.: NOC por correo"></div>
        <div class="field"><label>Tipo</label>
          <select class="select" name="type" id="channel-type">
            ${Object.keys(CHANNEL_FIELDS).map((t) => `<option value="${t}" ${type === t ? 'selected' : ''}>${esc(CHANNEL_LABELS[t] || t)}</option>`).join('')}
          </select></div>
        <div class="field full"><label>Estado</label>
          <label class="check"><input type="checkbox" name="enabled" ${!ch || ch.enabled ? 'checked' : ''}> Canal activo</label></div>
      </div>
      <div class="divider"></div>
      <div class="form-grid" id="channel-config">${channelConfigFieldsHTML(type, ch ? ch.config : {})}</div>
    </form>`;
  openModal(ch ? 'Editar canal' : 'Nuevo canal', body, {
    wide: true,
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="channel-save" data-id="${ch ? ch.id : ''}">${icon('check')} Guardar</button>`,
  });
  const sel = document.getElementById('channel-type');
  if (sel) {
    sel.addEventListener('change', () => {
      state.channelEditingType = sel.value;
      const box = document.getElementById('channel-config');
      if (box) box.innerHTML = channelConfigFieldsHTML(sel.value, {});
    });
  }
}

async function submitChannel(form) {
  const fd = new FormData(form);
  const type = String(fd.get('type') || '');
  const cfg = {};
  (CHANNEL_FIELDS[type] || []).forEach(([key, , , kind]) => {
    if (kind === 'bool') { cfg[key] = fd.get('ch_' + key) === 'on'; return; }
    const raw = fd.get('ch_' + key);
    if (raw === null) return;
    const value = String(raw).trim();
    if (kind === 'number') { if (value) cfg[key] = Number(value); return; }
    if (key === 'to') { cfg[key] = value.split(',').map((v) => v.trim()).filter(Boolean); return; }
    if (value) cfg[key] = value;
  });
  const id = Number(form.dataset.id || 0);
  const payload = {
    name: String(fd.get('name') || '').trim(),
    type,
    enabled: fd.get('enabled') === 'on',
    config: cfg,
  };
  if (!payload.name) throw new Error('El nombre es obligatorio');
  if (id) await api('/alerts/channels/' + id, { method: 'PUT', body: payload });
  else await api('/alerts/channels', { method: 'POST', body: payload });
  closeModal();
  toast(id ? 'Canal actualizado' : 'Canal creado', { type: 'ok' });
  state.channels = [];
  await renderAlerts('canales');
}

async function deleteChannel(id) {
  if (!confirm('¿Eliminar este canal de notificación?')) return;
  await api('/alerts/channels/' + id, { method: 'DELETE' });
  toast('Canal eliminado', { type: 'ok' });
  state.channels = [];
  await renderAlerts('canales');
}

async function testChannel(id, btn) {
  const previous = btn ? btn.innerHTML : '';
  if (btn) { btn.disabled = true; btn.innerHTML = 'Probando…'; }
  try {
    const res = await api('/alerts/channels/' + id + '/test', { method: 'POST' });
    if (res && res.ok) toast('Notificación de prueba enviada', { type: 'ok' });
    else toast((res && res.error) || 'El canal rechazó la prueba', { type: 'err' });
  } finally {
    if (btn) { btn.disabled = false; btn.innerHTML = previous; }
  }
  if (state.view === 'alerts' && state.alertsTab === 'canales') await renderAlerts('canales');
}

/* ------------------------------------------------------------------ */
/* Scans                                                               */
/* ------------------------------------------------------------------ */

async function renderScans() {
  setPageTitle('scans');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  await ensureNetworks();
  const res = await api('/scans?limit=60');
  const active = res.active || [];
  const items = res.items || [];
  const activeHTML = active.length ? active.map((s) => {
    const prog = state.scanActive[s.id] || s;
    const pct = Math.max(0, Math.min(100, prog.progress || prog.percent || (s.total ? (s.scanned / s.total) * 100 : 0)));
    return `<div class="card pad mb">
      <div class="spread">
        <div class="row"><span class="badge running"><i class="dot"></i>${esc(SCAN_LABELS[s.status] || s.status)}</span>
          <b class="mono">${esc(s.cidr)}</b><span class="muted small">${esc(s.profile || '')} · ${esc(s.trigger || 'manual')}</span></div>
        <button class="btn sm danger" data-action="scan-cancel" data-id="${s.id}">${icon('stop')} Cancelar</button>
      </div>
      <div class="row mt" style="gap:14px">
        <div class="progress" style="flex:1" data-scan-progress="${s.id}"><i style="width:${pct}%"></i></div>
        <span class="tiny muted mono" data-scan-label style="min-width:210px;text-align:right">${esc(prog.stage || s.stage || 'iniciando')} · ${fmtNum(prog.found || s.hosts_found)} encontrados</span>
      </div>
      <div class="tiny muted mt">${fmtNum(prog.scanned || s.scanned)} / ${fmtNum(prog.total || s.total)} direcciones · inicio ${fmtDate(s.started_at)}</div>
    </div>`;
  }).join('') : '';

  const rows = items.length ? items.map((s) => `
    <tr>
      <td class="mono">${esc(s.cidr)}</td>
      <td><span class="badge unknown">${esc(s.profile || '—')}</span></td>
      <td>${scanBadge(s.status)}</td>
      <td>${fmtNum(s.hosts_found)}</td>
      <td class="tiny muted">${fmtNum(s.scanned)}</td>
      <td class="tiny muted">${esc(s.trigger || 'manual')}</td>
      <td class="tiny muted">${fmtDate(s.started_at)}</td>
      <td class="tiny muted">${fmtDuration(s.started_at, s.finished_at)}</td>
      <td class="tiny" style="color:var(--red)">${esc(s.error || '')}</td>
    </tr>`).join('') : `<tr><td colspan="9"><div class="empty">${icon('scans')}Todavía no se ejecutó ningún escaneo</div></td></tr>`;

  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">El monitor periódico escanea las redes habilitadas según la configuración. Los escaneos manuales pueden solaparse hasta 3 a la vez.</div>
      <button class="btn primary" data-action="open-scan">${icon('radar')} Nuevo escaneo</button>
    </div>
    ${activeHTML}
    <div class="card">
      <div class="card-head"><h3>Historial</h3><span class="tiny muted">${fmtNum(res.total)} ejecuciones</span></div>
      <div class="table-wrap"><table>
        <thead><tr><th>Red</th><th>Perfil</th><th>Estado</th><th>Hallazgos</th><th>Analizadas</th><th>Origen</th><th>Inicio</th><th>Duración</th><th>Error</th></tr></thead>
        <tbody>${rows}</tbody>
      </table></div>
    </div>`;
}

async function openScanModal(preset) {
  await ensureNetworks();
  const nets = state.networks || [];
  const sel = preset && preset.network_id ? String(preset.network_id) : '';
  const body = `
    <form data-form="scan">
      <div class="form-grid">
        <div class="field full"><label>Red</label>
          <select class="select" name="network_id" id="scan-network">
            <option value="">Otra red (indicar CIDR)</option>
            ${nets.map((n) => `<option value="${n.id}" data-cidr="${esc(n.cidr)}" ${String(sel) === String(n.id) ? 'selected' : ''}>${esc(n.name || n.cidr)} · ${esc(n.cidr)}</option>`).join('')}
          </select></div>
        <div class="field"><label>CIDR</label>
          <input class="input mono" name="cidr" id="scan-cidr" placeholder="192.168.1.0/24" value="${esc(preset && preset.cidr ? preset.cidr : '')}"></div>
        <div class="field"><label>Perfil</label>
          <select class="select" name="profile" id="scan-profile">
            <option value="quick">Rápido — descubrimiento y pocos puertos</option>
            <option value="common" selected>Común — equilibrio recomendado</option>
            <option value="full">Completo — inventario profundo</option>
            <option value="custom">Personalizado</option>
          </select></div>
        <div class="field full hidden" id="scan-ports-field"><label>Puertos personalizados</label>
          <input class="input mono" name="ports" placeholder="22,80,443,8000-8100">
          <span class="help">Lista separada por comas; se admiten rangos.</span></div>
        <div class="field"><label>Timeout (ms)</label>
          <input class="input" name="timeout_ms" type="number" min="50" max="10000" placeholder="900"></div>
        <div class="field"><label>Opciones</label>
          <label class="check"><input type="checkbox" name="snmp"> Consultar SNMP</label>
          <label class="check"><input type="checkbox" name="deep"> Enriquecimiento profundo</label>
        </div>
      </div>
      <div class="notice mt"><span>Los escaneos masivos pueden tardar. Puedes cancelarlos desde esta pantalla en cualquier momento.</span></div>
    </form>`;
  openModal('Nuevo escaneo', body, {
    wide: true,
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="scan-start">${icon('play')} Iniciar</button>`,
  });
  const netSel = document.getElementById('scan-network');
  const cidr = document.getElementById('scan-cidr');
  if (netSel && cidr) {
    netSel.addEventListener('change', () => {
      const opt = netSel.selectedOptions[0];
      cidr.value = opt && opt.dataset.cidr ? opt.dataset.cidr : '';
    });
  }
  const prof = document.getElementById('scan-profile');
  if (prof) {
    const sync = () => {
      const field = document.getElementById('scan-ports-field');
      if (field) field.classList.toggle('hidden', prof.value !== 'custom');
    };
    prof.addEventListener('change', sync);
    sync();
  }
}

async function submitScan(form) {
  const fd = new FormData(form);
  const profile = String(fd.get('profile') || 'common');
  const networkID = Number(fd.get('network_id') || 0);
  const cidr = String(fd.get('cidr') || '').trim();
  if (!networkID && !cidr) throw new Error('Indica una red o un CIDR');
  const req = {
    site_id: Number(state.siteId) || 1,
    profile,
    network_id: networkID,
    cidr,
    snmp: fd.get('snmp') === 'on',
    deep: fd.get('deep') === 'on',
  };
  const timeout = String(fd.get('timeout_ms') || '').trim();
  if (timeout) req.timeout_ms = Number(timeout);
  if (profile === 'custom') {
    const raw = String(fd.get('ports') || '').trim();
    if (!raw) throw new Error('El perfil personalizado requiere puertos');
    const ports = [];
    raw.split(',').forEach((part) => {
      part = part.trim();
      if (!part) return;
      const m = part.match(/^(\d+)\s*-\s*(\d+)$/);
      if (m) {
        const a = Number(m[1]), b = Number(m[2]);
        if (a > b || b - a > 4096) throw new Error('Rango de puertos inválido: ' + part);
        for (let p = a; p <= b; p++) ports.push(p);
      } else {
        const p = Number(part);
        if (!Number.isInteger(p) || p < 1 || p > 65535) throw new Error('Puerto inválido: ' + part);
        ports.push(p);
      }
    });
    req.ports = ports;
  }
  const scan = await api('/scans', { method: 'POST', body: req });
  closeModal();
  toast(`Escaneo #${scan.id} iniciado en ${scan.cidr}`, { type: 'ok', title: 'Escaneo' });
  navTo('#/scans');
  if (state.view === 'scans') await renderScans();
}

async function cancelScan(id) {
  if (!confirm('¿Cancelar este escaneo? Se conservarán los dispositivos ya descubiertos.')) return;
  await api('/scans/' + id + '/cancel', { method: 'POST' });
  toast('Cancelación solicitada', { type: 'warn' });
  await renderScans();
}

/* ------------------------------------------------------------------ */
/* Reports                                                             */
/* ------------------------------------------------------------------ */

const REPORT_KINDS = [['inventory', 'Inventario'], ['executive', 'Ejecutivo'], ['alerts', 'Alertas']];
const REPORT_FORMATS = [['pdf', 'PDF'], ['csv', 'CSV'], ['json', 'JSON'], ['html', 'HTML']];

async function renderReports() {
  setPageTitle('reports');
  const content = document.getElementById('content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const res = await api('/reports');
  const items = res.items || [];
  const rows = items.length ? items.map((r) => `
    <tr>
      <td><div class="name-cell"><b>${esc(r.name)}</b><span>${esc((REPORT_KINDS.find(([k]) => k === r.kind) || [null, r.kind])[1])}</span></div></td>
      <td><span class="badge ${r.format === 'pdf' ? 'violet' : 'info'}">${esc((r.format || '').toUpperCase())}</span></td>
      <td class="tiny muted">${fmtBytes(r.size)}</td>
      <td class="tiny muted">${fmtDate(r.created_at)}</td>
      <td class="right nowrap">
        <a class="btn sm" href="/api/v1/reports/${r.id}/download">${icon('download')} Descargar</a>
        <button class="btn sm danger" data-action="report-delete" data-id="${r.id}">${icon('trash')}</button>
      </td>
    </tr>`).join('') : `<tr><td colspan="5"><div class="empty">${icon('reports')}Sin informes generados</div></td></tr>`;
  content.innerHTML = `
    <div class="spread mb">
      <div class="muted small">Los informes se generan en el servidor y quedan disponibles para descargar. El PDF incluye portada, resumen ejecutivo e inventario completo.</div>
      <div class="row">
        <a class="btn sm" href="/api/v1/hosts/export?format=csv" download>${icon('download')} Inventario CSV</a>
        <button class="btn primary" data-action="report-new">${icon('plus')} Generar informe</button>
      </div>
    </div>
    <div class="card">
      <div class="card-head"><h3>Informes</h3><span class="tiny muted">${items.length} archivos</span></div>
      <div class="table-wrap"><table>
        <thead><tr><th>Informe</th><th>Formato</th><th>Tamaño</th><th>Creado</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
      </table></div>
    </div>`;
}

function openReportModal() {
  const body = `
    <form data-form="report">
      <div class="form-grid">
        <div class="field"><label>Tipo</label>
          <select class="select" name="kind">${REPORT_KINDS.map(([v, l]) => `<option value="${v}">${l}</option>`).join('')}</select></div>
        <div class="field"><label>Formato</label>
          <select class="select" name="format">${REPORT_FORMATS.map(([v, l]) => `<option value="${v}">${l}</option>`).join('')}</select></div>
        <div class="field full"><label>Título (opcional)</label>
          <input class="input" name="title" maxlength="120" placeholder="Informe de red — sede principal"></div>
      </div>
      <div class="notice mt"><span>El informe incluye el inventario actual y las alertas de los últimos 7 días.</span></div>
    </form>`;
  openModal('Generar informe', body, {
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="report-create">${icon('download')} Generar</button>`,
  });
}

async function submitReport(form) {
  const fd = new FormData(form);
  const rec = await api('/reports', {
    method: 'POST',
    body: {
      site_id: Number(state.siteId) || 0,
      kind: String(fd.get('kind') || 'inventory'),
      format: String(fd.get('format') || 'pdf'),
      title: String(fd.get('title') || '').trim(),
    },
  });
  closeModal();
  toast(rec.name || 'Informe generado', { type: 'ok', title: 'Informe listo' });
  await renderReports();
}

async function deleteReport(id) {
  if (!confirm('¿Eliminar este informe y su archivo?')) return;
  await api('/reports/' + id, { method: 'DELETE' });
  toast('Informe eliminado', { type: 'ok' });
  await renderReports();
}

/* ------------------------------------------------------------------ */
/* Settings                                                            */
/* ------------------------------------------------------------------ */

const SETTINGS_TABS = [
  ['sedes', 'Sedes'],
  ['general', 'General'],
  ['escaneo', 'Escaneo'],
  ['alertas', 'Alertas'],
  ['integraciones', 'Integraciones'],
  ['seguridad', 'Seguridad'],
  ['almacenamiento', 'Almacenamiento'],
  ['redes', 'Redes'],
  ['usuarios', 'Usuarios'],
  ['copias', 'Copias'],
  ['sistema', 'Sistema'],
];

function getPath(obj, path) {
  return path.split('.').reduce((o, k) => (o === undefined || o === null ? undefined : o[k]), obj);
}

function setPath(obj, path, value) {
  const parts = path.split('.');
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    if (typeof cur[parts[i]] !== 'object' || cur[parts[i]] === null) cur[parts[i]] = {};
    cur = cur[parts[i]];
  }
  cur[parts[parts.length - 1]] = value;
}

function cfgInput(label, name, value, opts = {}) {
  const help = opts.help ? `<span class="help">${esc(opts.help)}</span>` : '';
  if (opts.type === 'checkbox') {
    return `<div class="field"><label>${esc(label)}</label>
      <label class="check"><input type="checkbox" name="${name}" ${value ? 'checked' : ''}> ${esc(opts.hint || 'Activado')}</label>${help}</div>`;
  }
  if (opts.type === 'select') {
    return `<div class="field"><label>${esc(label)}</label>
      <select class="select" name="${name}">${(opts.options || []).map(([v, l]) =>
        `<option value="${esc(v)}" ${String(value) === String(v) ? 'selected' : ''}>${esc(l)}</option>`).join('')}</select>${help}</div>`;
  }
  return `<div class="field"><label>${esc(label)}</label>
    <input class="input" name="${name}" ${opts.type === 'number' ? 'type="number"' : ''} ${opts.min !== undefined ? `min="${opts.min}"` : ''} ${opts.max !== undefined ? `max="${opts.max}"` : ''} value="${esc(value === undefined || value === null ? '' : value)}" placeholder="${esc(opts.placeholder || '')}" ${opts.mono ? 'style="font-family:var(--mono)"' : ''}>${help}</div>`;
}

function configFormBody(cfg) {
  const s = cfg.server || {}, sc = cfg.scan || {}, mon = cfg.monitor || {}, al = cfg.alerts || {},
    intg = cfg.integrations || {}, st = cfg.storage || {}, sec = cfg.security || {}, log = cfg.log || {};
  const tab = state.settingsTab;
  let fields = '';
  let extra = '';
  if (tab === 'general') {
    fields = `<div class="form-grid">
      ${cfgInput('Dirección de escucha', 'server.listen', s.listen, { placeholder: '127.0.0.1:8080', help: 'Cambiarla requiere reiniciar el servicio.' })}
      ${cfgInput('URL pública', 'server.public_url', s.public_url, { placeholder: 'https://netdiag.example.com', help: 'Se usa para enlaces y cookies seguras.' })}
      ${cfgInput('Certificado TLS', 'server.tls_cert', s.tls_cert, { placeholder: '/ruta/fullchain.pem', mono: true })}
      ${cfgInput('Clave TLS', 'server.tls_key', s.tls_key, { placeholder: '/ruta/privkey.pem', mono: true })}
      ${cfgInput('Tamaño máximo de cuerpo (MB)', 'server.max_body_mb', s.max_body_mb, { type: 'number', min: 1, max: 100 })}
      ${cfgInput('Nivel de registro', 'log.level', log.level, { type: 'select', options: [['debug', 'Debug'], ['info', 'Info'], ['warn', 'Aviso'], ['error', 'Error']] })}
      ${cfgInput('Formato de registro', 'log.format', log.format, { type: 'select', options: [['text', 'Texto'], ['json', 'JSON']] })}
      <div class="field"><label>Cabeceras de seguridad</label>
        <label class="check"><input type="checkbox" name="server.enable_csp" ${s.enable_csp ? 'checked' : ''}> Content-Security-Policy</label>
        <label class="check"><input type="checkbox" name="server.enable_hsts" ${s.enable_hsts ? 'checked' : ''}> HSTS</label></div>
    </div>
    <div class="notice mt"><span>Los cambios de dirección, TLS y registro se aplican al reiniciar el proceso. El resto se aplica en caliente.</span></div>`;
  } else if (tab === 'escaneo') {
    fields = `<div class="form-grid">
      ${cfgInput('Perfil por defecto', 'scan.profile', sc.profile, { type: 'select', options: [['quick', 'Rápido'], ['common', 'Común'], ['full', 'Completo'], ['custom', 'Personalizado']] })}
      ${cfgInput('Timeout por host', 'scan.timeout', sc.timeout, { placeholder: '900ms', help: 'Entre 50ms y 10s.' })}
      ${cfgInput('Concurrencia de hosts', 'scan.concurrency', sc.concurrency, { type: 'number', min: 1, max: 1024 })}
      ${cfgInput('Concurrencia de puertos', 'scan.port_concurrency', sc.port_concurrency, { type: 'number', min: 1, max: 2048 })}
      ${cfgInput('Máximo de hosts', 'scan.max_hosts', sc.max_hosts, { type: 'number', min: 1, max: 65536 })}
      ${cfgInput('Timeout SNMP', 'scan.snmp_timeout', sc.snmp_timeout, { placeholder: '1s' })}
      ${cfgInput('Versión SNMP', 'scan.snmp_version', sc.snmp_version, { type: 'select', options: [['1', 'v1'], ['2c', 'v2c'], ['3', 'v3']] })}
      ${cfgInput('Comunidades SNMP', 'scan.snmp_communities', (sc.snmp_communities || []).join(', '), { placeholder: 'public, privada', help: 'Separadas por comas.' })}
      ${cfgInput('Usuario SNMPv3', 'scan.snmp_v3_user', sc.snmp_v3_user)}
      ${cfgInput('Auth SNMPv3', 'scan.snmp_v3_auth', sc.snmp_v3_auth === '********' ? '' : sc.snmp_v3_auth, { placeholder: 'dejar en blanco para conservar' })}
      ${cfgInput('Priv SNMPv3', 'scan.snmp_v3_priv', sc.snmp_v3_priv === '********' ? '' : sc.snmp_v3_priv, { placeholder: 'dejar en blanco para conservar' })}
      ${cfgInput('Protocolo auth', 'scan.snmp_v3_auth_proto', sc.snmp_v3_auth_proto, { type: 'select', options: [['md5', 'MD5'], ['sha', 'SHA-1'], ['sha224', 'SHA-224'], ['sha256', 'SHA-256'], ['sha384', 'SHA-384'], ['sha512', 'SHA-512']] })}
      ${cfgInput('Protocolo priv', 'scan.snmp_v3_priv_proto', sc.snmp_v3_priv_proto, { type: 'select', options: [['des', 'DES'], ['aes', 'AES-128'], ['aes192', 'AES-192'], ['aes256', 'AES-256']] })}
      <div class="field"><label>Comportamiento</label>
        <label class="check"><input type="checkbox" name="scan.resolve_dns" ${sc.resolve_dns ? 'checked' : ''}> Resolver DNS inverso</label>
        <label class="check"><input type="checkbox" name="scan.banner_grab" ${sc.banner_grab ? 'checked' : ''}> Capturar banners</label>
        <label class="check"><input type="checkbox" name="scan.snmp" ${sc.snmp ? 'checked' : ''}> SNMP activado por defecto</label>
        <label class="check"><input type="checkbox" name="scan.auto_discover_l2" ${sc.auto_discover_l2 ? 'checked' : ''}> Descubrir topología L2</label>
        <label class="check"><input type="checkbox" name="scan.allow_public" ${sc.allow_public ? 'checked' : ''}> Permitir redes públicas</label></div>
    </div>
    <div class="divider"></div>
    <h3 style="margin:0 0 12px;font-size:13px">Monitor periódico</h3>
    <div class="form-grid">
      <div class="field"><label>Estado</label>
        <label class="check"><input type="checkbox" name="monitor.enabled" ${mon.enabled ? 'checked' : ''}> Monitor activo</label></div>
      ${cfgInput('Intervalo', 'monitor.interval', mon.interval, { placeholder: '5m', help: 'Mínimo 30s.' })}
      ${cfgInput('Intervalo de escaneo profundo', 'monitor.deep_interval', mon.deep_interval, { placeholder: '6h' })}
      ${cfgInput('Redes monitorizadas', 'monitor.network_ids', (mon.network_ids || []).join(', '), { placeholder: 'todas', help: 'IDs separados por comas; vacío = todas.' })}
    </div>`;
  } else if (tab === 'alertas') {
    fields = `<div class="form-grid">
      <div class="field"><label>Motor de alertas</label>
        <label class="check"><input type="checkbox" name="alerts.enabled" ${al.enabled ? 'checked' : ''}> Alertas activadas</label></div>
      ${cfgInput('Timeout de webhooks', 'alerts.webhook_timeout', al.webhook_timeout, { placeholder: '10s' })}
      ${cfgInput('Máximo por minuto', 'alerts.max_per_minute', al.max_per_minute, { type: 'number', min: 1, max: 600, help: 'Límite global de notificaciones salientes.' })}
      <div class="field"><label>Destinos privados</label>
        <label class="check"><input type="checkbox" name="alerts.allow_private_targets" ${al.allow_private_targets ? 'checked' : ''}> Permitir webhooks a redes privadas</label>
        <span class="help">Desactivado bloquea 10/8, 172.16/12, 192.168/16, loopback y metadatos cloud.</span></div>
      <div class="field"><label>SMTP</label>
        <label class="check"><input type="checkbox" name="alerts.smtp_insecure" ${al.smtp_insecure ? 'checked' : ''}> Omitir verificación TLS (no recomendado)</label></div>
    </div>
    <div class="notice mt"><span>Los canales y las reglas se gestionan desde la sección Alertas.</span></div>`;
  } else if (tab === 'integraciones') {
    fields = `<div class="form-grid">
      <div class="field"><label>Motor de integraciones</label>
        <label class="check"><input type="checkbox" name="integrations.enabled" ${intg.enabled !== false ? 'checked' : ''}> Exportar eventos</label></div>
      ${cfgInput('Timeout de entrega', 'integrations.timeout', intg.timeout || '10s', { placeholder: '10s', help: 'Entre 1s y 1m.' })}
      ${cfgInput('Máximo de eventos por minuto', 'integrations.max_per_minute', intg.max_per_minute || 60, { type: 'number', min: 1, max: 10000, help: 'Límite global compartido por todas las integraciones.' })}
      ${cfgInput('Tamaño de la cola', 'integrations.queue_size', intg.queue_size || 512, { type: 'number', min: 16, max: 100000, help: 'Eventos en memoria; si se llena se descartan con aviso.' })}
      <div class="field"><label>Destinos privados</label>
        <label class="check"><input type="checkbox" name="integrations.allow_private_targets" ${intg.allow_private_targets ? 'checked' : ''}> Permitir Elasticsearch/Kafka/Redis/syslog locales</label>
        <span class="help">Necesario para 127.0.0.1, 10/8, 172.16/12 y 192.168/16. Bloquea siempre metadatos cloud.</span></div>
    </div>`;
  } else if (tab === 'seguridad') {
    fields = `<div class="form-grid">
      <div class="field"><label>Autenticación</label>
        <label class="check"><input type="checkbox" name="security.auth" ${sec.auth ? 'checked' : ''}> Exigir inicio de sesión</label>
        <span class="help">Si se desactiva, la instancia queda abierta a cualquiera que alcance el puerto.</span></div>
      ${cfgInput('Duración de sesión (horas)', 'security.session_hours', sec.session_hours, { type: 'number', min: 1, max: 720 })}
      ${cfgInput('Intentos de login / minuto', 'security.login_rate', sec.login_rate, { type: 'number', min: 1, max: 100 })}
      ${cfgInput('Peticiones API / minuto', 'security.api_rate', sec.api_rate, { type: 'number', min: 10, max: 100000 })}
      ${cfgInput('Hosts permitidos', 'security.allowed_hosts', (sec.allowed_hosts || []).join(', '), { placeholder: 'netdiag.example.com', help: 'Separados por comas; vacío = cualquiera.' })}
      ${cfgInput('Token de métricas', 'security.metrics_token', sec.metrics_token === '********' ? '' : sec.metrics_token, { placeholder: 'dejar en blanco para conservar', help: 'Si se define, /metrics exige Authorization: Bearer <token>.' })}
      <div class="field"><label>Proxy inverso</label>
        <label class="check"><input type="checkbox" name="security.trust_proxy" ${sec.trust_proxy ? 'checked' : ''}> Confiar en X-Forwarded-For</label></div>
    </div>`;
    extra = `<div class="card pad mt">
      <h3 style="margin:0 0 12px;font-size:13px">Cambiar contraseña</h3>
      <form data-form="password" class="form-grid">
        <div class="field"><label>Contraseña actual</label><input class="input" type="password" name="current_password" autocomplete="current-password" required></div>
        <div class="field"><label>Nueva contraseña</label><input class="input" type="password" name="new_password" autocomplete="new-password" minlength="10" required></div>
        <div class="field full"><button class="btn" type="submit">Actualizar contraseña</button>
          <span class="help">Al cambiarla se cierran todas las sesiones, incluida esta.</span></div>
      </form>
    </div>`;
  } else if (tab === 'almacenamiento') {
    fields = `<div class="form-grid">
      ${cfgInput('Directorio de datos', 'storage.data_dir', st.data_dir, { mono: true, help: 'Requiere reinicio; no lo cambies con datos existentes.' })}
      ${cfgInput('Base de datos', 'storage.db', st.db, { mono: true, help: 'Ruta absoluta o relativa al directorio de datos.' })}
      ${cfgInput('Retención de alertas (días)', 'storage.retention_days', st.retention_days, { type: 'number', min: 1 })}
      ${cfgInput('Retención de métricas (días)', 'storage.metrics_days', st.metrics_days, { type: 'number', min: 1 })}
      ${cfgInput('Retención de auditoría (días)', 'storage.audit_days', st.audit_days, { type: 'number', min: 1 })}
      ${cfgInput('Máximo de escaneos', 'storage.max_scans', st.max_scans, { type: 'number', min: 10 })}
      ${cfgInput('Copia automática', 'storage.backup_enabled', st.backup_enabled, { type: 'checkbox', hint: 'Activar copias periódicas' })}
      ${cfgInput('Directorio de copias', 'storage.backup_dir', st.backup_dir, { mono: true, help: 'Por defecto <data_dir>/backups.' })}
      ${cfgInput('Intervalo entre copias', 'storage.backup_interval', st.backup_interval, { placeholder: '24h', help: 'Mínimo 15m.' })}
      ${cfgInput('Copias a conservar', 'storage.backup_keep', st.backup_keep, { type: 'number', min: 1, max: 365 })}
    </div>`;
  }
  const wrapForm = ['general', 'escaneo', 'alertas', 'integraciones', 'seguridad', 'almacenamiento'].includes(tab);
  if (!wrapForm) return '';
  return `<form data-form="config" data-tab="${tab}">
    ${fields}
    <div class="spread mt"><span class="tiny muted">Los cambios se guardan en el archivo de configuración.</span>
      <button class="btn primary" type="submit">${icon('check')} Guardar cambios</button></div>
  </form>${extra}`;
}

function readConfigForm(form, current) {
  const next = JSON.parse(JSON.stringify(current || {}));
  form.querySelectorAll('[name]').forEach((el) => {
    const name = el.getAttribute('name');
    if (!name) return;
    if (el.type === 'checkbox') { setPath(next, name, el.checked); return; }
    const value = el.value;
    if (name === 'scan.snmp_communities' || name === 'security.allowed_hosts') {
      setPath(next, name, String(value).split(',').map((v) => v.trim()).filter(Boolean));
      return;
    }
    if (name === 'monitor.network_ids') {
      setPath(next, name, String(value).split(',').map((v) => Number(v.trim())).filter((n) => Number.isInteger(n) && n > 0));
      return;
    }
    if (['server.max_body_mb', 'scan.concurrency', 'scan.port_concurrency', 'scan.max_hosts',
      'alerts.max_per_minute', 'integrations.max_per_minute', 'integrations.queue_size',
      'security.session_hours', 'security.login_rate', 'security.api_rate',
      'storage.retention_days', 'storage.metrics_days', 'storage.audit_days', 'storage.max_scans',
      'storage.backup_keep'].includes(name)) {
      setPath(next, name, Number(value));
      return;
    }
    if (name === 'scan.snmp_v3_auth' || name === 'scan.snmp_v3_priv' || name === 'security.metrics_token') {
      if (String(value).trim() === '') return; // keep stored secret
    }
    setPath(next, name, value);
  });
  return next;
}

async function submitConfig(form) {
  const next = readConfigForm(form, state.settingsCfg);
  await api('/system/config', { method: 'PUT', body: next });
  state.settingsCfg = next;
  toast('Configuración guardada', { type: 'ok' });
  await renderSettings();
}

async function submitPassword(form) {
  const fd = new FormData(form);
  await api('/auth/password', {
    method: 'POST',
    body: { current_password: String(fd.get('current_password') || ''), new_password: String(fd.get('new_password') || '') },
    noAuthRedirect: true,
  });
  toast('Contraseña actualizada; vuelve a iniciar sesión', { type: 'ok' });
  state.me = null;
  state.csrf = '';
  setTimeout(() => showLogin(Object.assign({}, state.auth, { authenticated: false })), 900);
}

async function renderSettings() {
  setPageTitle('settings');
  const content = document.getElementById('content');
  const tab = state.settingsTab || 'general';
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const tabsHTML = `<div class="tabs" style="margin-top:0">
    ${SETTINGS_TABS.map(([key, label]) => `<div class="tab ${tab === key ? 'active' : ''}" data-action="settings-tab" data-tab="${key}">${label}</div>`).join('')}
  </div>`;
  let body = '';
  if (tab === 'sedes') {
    body = await settingsSitesBody();
  } else if (tab === 'redes') {
    body = await settingsNetworksBody();
  } else if (tab === 'usuarios') {
    body = await settingsUsersBody();
  } else if (tab === 'copias') {
    body = await settingsBackupsBody();
  } else if (tab === 'sistema') {
    body = await settingsSystemBody();
  } else {
    const res = await api('/system/config');
    state.settingsCfg = res.config;
    body = configFormBody(res.config);
    if (tab === 'integraciones') body += await settingsIntegrationsBody();
  }
  content.innerHTML = tabsHTML + body;
}

/* ---------------- external integrations ---------------- */

const INTEGRATION_LABELS = {
  elasticsearch: 'Elasticsearch', syslog: 'Syslog', webhook: 'Webhook',
  slack: 'Slack', discord: 'Discord', msteams: 'MS Teams', telegram: 'Telegram',
  pagerduty: 'PagerDuty', kafka: 'Kafka REST', redis: 'Redis Streams',
};

const INTEGRATION_FIELDS = {
  elasticsearch: [
    ['url', 'URL', 'http://localhost:9200', 'text'],
    ['index', 'Índice', 'netdiag-events', 'text'],
    ['api_key', 'API key (opcional)', 'dejar en blanco para conservar', 'secret'],
    ['username', 'Usuario (opcional)', '', 'secret'],
    ['password', 'Contraseña (opcional)', 'dejar en blanco para conservar', 'secret'],
  ],
  syslog: [
    ['host', 'Servidor', '127.0.0.1', 'text'],
    ['port', 'Puerto', '514', 'number'],
    ['protocol', 'Transporte', 'udp', 'select:udp,tcp'],
    ['app_name', 'Nombre de aplicación', 'netdiag', 'text'],
  ],
  webhook: [
    ['url', 'URL del webhook', 'https://example.com/hook', 'text'],
    ['secret', 'Secreto HMAC (opcional)', 'dejar en blanco para conservar', 'secret'],
  ],
  slack: [['url', 'Webhook de Slack', 'https://hooks.slack.com/services/…', 'secret']],
  discord: [['url', 'Webhook de Discord', 'https://discord.com/api/webhooks/…', 'secret']],
  msteams: [['url', 'Webhook de MS Teams', 'https://outlook.office.com/webhook/…', 'secret']],
  telegram: [
    ['token', 'Token del bot', '123456:ABC…', 'secret'],
    ['chat_id', 'Chat ID', '-1001234567890', 'text'],
  ],
  pagerduty: [
    ['routing_key', 'Routing key', 'Clave de integración', 'secret'],
    ['url', 'URL de eventos (opcional)', 'https://events.pagerduty.com/v2/enqueue', 'text'],
  ],
  kafka: [
    ['url', 'URL del REST Proxy', 'http://localhost:8082', 'text'],
    ['topic', 'Topic', 'netdiag.events', 'text'],
    ['token', 'Token Bearer (opcional)', 'dejar en blanco para conservar', 'secret'],
  ],
  redis: [
    ['addr', 'Dirección', '127.0.0.1:6379', 'text'],
    ['stream', 'Stream', 'netdiag:events', 'text'],
    ['username', 'Usuario ACL (opcional)', '', 'secret'],
    ['password', 'Contraseña (opcional)', 'dejar en blanco para conservar', 'secret'],
    ['max_len', 'MAXLEN approximado', '10000', 'number'],
  ],
};

function integrationConfigFieldsHTML(type, cfg) {
  cfg = cfg || {};
  return (INTEGRATION_FIELDS[type] || []).map(([key, label, placeholder, kind]) => {
    const value = cfg[key];
    if (kind.startsWith('select:')) {
      const options = kind.slice(7).split(',');
      return `<div class="field"><label>${esc(label)}</label>
        <select class="select" name="ig_${key}">${options.map((o) => `<option value="${o}" ${String(value) === o ? 'selected' : ''}>${o.toUpperCase()}</option>`).join('')}</select></div>`;
    }
    if (kind === 'secret') {
      return `<div class="field full"><label>${esc(label)}</label>
        <input class="input" name="ig_${key}" autocomplete="new-password" value="${esc(value === '********' ? '' : value || '')}" placeholder="${esc(placeholder)}">
        <span class="help">Se cifra en la base de datos; déjalo en blanco para conservar el valor actual.</span></div>`;
    }
    return `<div class="field"><label>${esc(label)}</label>
      <input class="input" name="ig_${key}" ${kind === 'number' ? 'type="number"' : ''} value="${esc(value === undefined || value === null ? '' : value)}" placeholder="${esc(placeholder)}"></div>`;
  }).join('');
}

async function settingsIntegrationsBody() {
  if (!state.integrationsLoaded) {
    const res = await api('/integrations');
    state.integrations = res.items || [];
    state.integrationMeta = { types: res.types || [], secret_fields: res.secret_fields || {} };
    state.integrationsLoaded = true;
  }
  const items = state.integrations || [];
  const rows = items.length ? items.map((it) => `
    <tr>
      <td><span class="toggle ${it.enabled ? 'on' : ''}" data-action="integration-toggle" data-id="${it.id}" data-enabled="${it.enabled}" style="cursor:pointer"></span></td>
      <td><div class="name-cell"><b>${esc(it.name)}</b><span>${esc(INTEGRATION_LABELS[it.type] || it.type)}</span></div></td>
      <td>${it.has_secret ? '<span class="badge up">Credencial cifrada</span>' : '<span class="badge unknown">Sin secreto</span>'}</td>
      <td class="tiny muted">${it.last_test_at ? fmtDate(it.last_test_at) : 'Nunca'}</td>
      <td class="tiny" style="color:var(--red)">${esc(it.last_error || '')}</td>
      <td class="right nowrap">
        <button class="btn sm" data-action="integration-test" data-id="${it.id}">${icon('test')} Probar</button>
        <button class="btn sm" data-action="integration-edit" data-id="${it.id}">${icon('edit')} Editar</button>
        <button class="btn sm danger" data-action="integration-delete" data-id="${it.id}">${icon('trash')}</button>
      </td>
    </tr>`).join('') : `<tr><td colspan="6"><div class="empty">${icon('plus')}Sin integraciones configuradas</div></td></tr>`;
  return `<div class="card mt">
    <div class="spread mb">
      <div class="muted small">Exporta alertas y escaneos a sistemas externos. Los secretos se cifran con AES-256-GCM y nunca se muestran después de guardarlos.</div>
      <button class="btn primary" data-action="integration-new">${icon('plus')} Nueva integración</button>
    </div>
    <div class="table-wrap"><table>
      <thead><tr><th>Activo</th><th>Integración</th><th>Credencial</th><th>Última prueba</th><th>Último error</th><th></th></tr></thead>
      <tbody>${rows}</tbody>
    </table></div>
  </div>`;
}

function openIntegrationModal(id) {
  const it = id ? (state.integrations || []).find((x) => x.id === id) : null;
  const type = it ? it.type : 'elasticsearch';
  state.integrationEditingType = type;
  const body = `<form data-form="integration" data-id="${it ? it.id : ''}">
    <div class="form-grid">
      <div class="field"><label>Nombre</label>
        <input class="input" name="name" required maxlength="80" value="${esc(it ? it.name : '')}" placeholder="Ej.: Elasticsearch local"></div>
      <div class="field"><label>Tipo</label>
        <select class="select" name="type" id="integration-type">
          ${Object.keys(INTEGRATION_FIELDS).map((t) => `<option value="${t}" ${type === t ? 'selected' : ''}>${esc(INTEGRATION_LABELS[t] || t)}</option>`).join('')}
        </select></div>
      <div class="field full"><label>Estado</label>
        <label class="check"><input type="checkbox" name="enabled" ${!it || it.enabled ? 'checked' : ''}> Integración activa</label></div>
    </div>
    <div class="divider"></div>
    <div class="form-grid" id="integration-config">${integrationConfigFieldsHTML(type, it ? it.config : {})}</div>
  </form>`;
  openModal(it ? 'Editar integración' : 'Nueva integración', body, {
    wide: true,
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="integration-save" data-id="${it ? it.id : ''}">${icon('check')} Guardar</button>`,
  });
  const sel = document.getElementById('integration-type');
  if (sel) sel.addEventListener('change', () => {
    const box = document.getElementById('integration-config');
    if (box) box.innerHTML = integrationConfigFieldsHTML(sel.value, {});
  });
}

async function submitIntegration(form) {
  const fd = new FormData(form);
  const type = String(fd.get('type') || '');
  const cfg = {};
  (INTEGRATION_FIELDS[type] || []).forEach(([key, , , kind]) => {
    if (kind.startsWith('select:')) { cfg[key] = String(fd.get('ig_' + key) || ''); return; }
    const raw = fd.get('ig_' + key);
    if (raw === null) return;
    const value = String(raw).trim();
    if (kind === 'number') { if (value) cfg[key] = Number(value); return; }
    if (value) cfg[key] = value;
  });
  const id = Number(form.dataset.id || 0);
  const payload = {
    name: String(fd.get('name') || '').trim(), type,
    enabled: fd.get('enabled') === 'on', config: cfg,
  };
  if (!payload.name) throw new Error('El nombre es obligatorio');
  if (id) await api('/integrations/' + id, { method: 'PUT', body: payload });
  else await api('/integrations', { method: 'POST', body: payload });
  closeModal();
  toast(id ? 'Integración actualizada' : 'Integración creada', { type: 'ok' });
  state.integrationsLoaded = false;
  await renderSettings();
}

async function deleteIntegration(id) {
  if (!confirm('¿Eliminar esta integración?')) return;
  await api('/integrations/' + id, { method: 'DELETE' });
  toast('Integración eliminada', { type: 'ok' });
  state.integrationsLoaded = false;
  await renderSettings();
}

async function toggleIntegration(id, enabled) {
  const it = (state.integrations || []).find((x) => x.id === id);
  if (!it) return;
  await api('/integrations/' + id, { method: 'PUT', body: { enabled: !enabled } });
  toast(enabled ? 'Integración deshabilitada' : 'Integración habilitada', { type: 'ok' });
  state.integrationsLoaded = false;
  await renderSettings();
}

async function saveIntegrationFromModal(id) {
  const form = document.querySelector('form[data-form="integration"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="integration-save"]');
  if (btn) btn.disabled = true;
  try {
    await submitIntegration(form);
  } catch (e) {
    if (btn) btn.disabled = false;
    throw e;
  }
}

async function testIntegration(id, btn) {
  const previous = btn ? btn.innerHTML : '';
  if (btn) { btn.disabled = true; btn.innerHTML = 'Probando…'; }
  try {
    const res = await api('/integrations/' + id + '/test', { method: 'POST' });
    if (res && res.ok) toast('Evento de prueba enviado', { type: 'ok' });
    else toast((res && res.error) || 'El destino rechazó la prueba', { type: 'err' });
  } finally {
    if (btn) { btn.disabled = false; btn.innerHTML = previous; }
    state.integrationsLoaded = false;
    await renderSettings();
  }
}

function siteOptions(selected, includeAll = false) {
  const all = includeAll ? `<option value="0" ${!selected ? 'selected' : ''}>Todas las sedes</option>` : '';
  return all + (state.sites || []).map((s) =>
    `<option value="${s.id}" ${Number(selected) === Number(s.id) ? 'selected' : ''}>${esc(s.name)}</option>`
  ).join('');
}

async function settingsSitesBody() {
  await loadSites(true);
  const sites = state.sites || [];
  const rows = sites.map((s) => {
    const active = Number(s.id) === Number(state.siteId);
    return `<tr>
      <td><div class="name-cell"><b>${esc(s.name)}</b><span class="mono">${esc(s.slug)}</span></div></td>
      <td class="tiny muted">${esc(s.description || '—')}</td>
      <td class="right">${s.network_count || 0}</td>
      <td class="right">${s.host_count || 0}</td>
      <td class="right">${s.up_count || 0}</td>
      <td class="tiny muted">${s.last_scan_at ? ago(s.last_scan_at) : 'Nunca'}</td>
      <td class="right nowrap">
        ${active ? '<span class="badge up">Activa</span>' : `<button class="btn sm" data-action="site-activate" data-id="${s.id}">${esc(tr('Activar'))}</button>`}
        ${isAdmin() ? `<button class="btn sm" data-action="site-edit" data-id="${s.id}">${icon('edit')}</button>
        <button class="btn sm" data-action="site-export" data-id="${s.id}" title="Exportar">${icon('download')}</button>
        <button class="btn sm" data-action="site-import" data-id="${s.id}" title="Importar">${icon('plus')}</button>
        <button class="btn sm danger" data-action="site-delete" data-id="${s.id}" title="Eliminar">${icon('trash')}</button>` : ''}
      </td>
    </tr>`;
  }).join('');
  return `
    <div class="spread mb">
      <div class="muted small">Cada sede tiene sus propias redes, dispositivos, escaneos e informes. El mismo CIDR puede existir en varias sedes. Selecciona la sede activa en la barra superior.</div>
      ${isAdmin() ? `<button class="btn primary" data-action="site-new">${icon('plus')} Nueva sede</button>` : ''}
    </div>
    <div class="card"><div class="table-wrap"><table>
      <thead><tr><th>Sede</th><th>Descripción</th><th class="right">Redes</th><th class="right">Dispositivos</th><th class="right">En línea</th><th>Último escaneo</th><th></th></tr></thead>
      <tbody>${rows || `<tr><td colspan="7"><div class="empty">${icon('network')}Sin sedes</div></td></tr>`}</tbody>
    </table></div></div>
    <div class="notice mt"><span>Las copias de seguridad incluyen todas las sedes. Puedes exportar una sede como paquete JSON e importarlo en otra instancia o en otra sede.</span></div>`;
}

function openSiteModal(id) {
  const site = (state.sites || []).find((s) => Number(s.id) === Number(id)) || null;
  const body = `
    <form data-form="site" data-id="${site ? site.id : ''}">
      <div class="form-grid">
        <div class="field full"><label>Nombre</label>
          <input class="input" name="name" required maxlength="80" value="${esc(site ? site.name : '')}" placeholder="Ej.: Oficina norte"></div>
        <div class="field full"><label>Descripción</label>
          <input class="input" name="description" maxlength="200" value="${esc(site ? site.description || '' : '')}" placeholder="Ubicación, responsable, alcance…"></div>
      </div>
    </form>`;
  openModal(site ? 'Editar sede' : 'Nueva sede', body, {
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="site-save" data-id="${site ? site.id : ''}">${icon('check')} Guardar</button>`,
  });
}

async function saveSiteFromModal(id) {
  const form = document.querySelector('form[data-form="site"]');
  if (!form) return;
  const fd = new FormData(form);
  const payload = {
    name: String(fd.get('name') || '').trim(),
    description: String(fd.get('description') || '').trim(),
  };
  if (!payload.name) throw new Error('El nombre de la sede es obligatorio');
  if (id) await api('/sites/' + id, { method: 'PUT', body: payload });
  else await api('/sites', { method: 'POST', body: payload });
  closeModal();
  toast(id ? 'Sede actualizada' : 'Sede creada', { type: 'ok' });
  await loadSites(true);
  await renderSettings();
}

async function deleteSite(id) {
  const site = (state.sites || []).find((s) => Number(s.id) === Number(id));
  if (!site) return;
  if (!confirm(`¿Eliminar la sede "${site.name}"? Sus redes, dispositivos y escaneos se moverán a otra sede.`)) return;
  const others = (state.sites || []).filter((s) => Number(s.id) !== Number(id));
  const target = others.find((s) => Number(s.id) !== 1) || others[0];
  const moveTo = target ? target.id : 0;
  await api('/sites/' + id + '?move_to=' + moveTo, { method: 'DELETE' });
  if (Number(state.siteId) === Number(id)) {
    state.siteId = Number(target ? target.id : 1);
    try { localStorage.setItem('netdiag-site', String(state.siteId)); } catch {}
  }
  toast('Sede eliminada', { type: 'ok' });
  await loadSites(true);
  await renderSettings();
}

async function exportSite(id) {
  const site = (state.sites || []).find((s) => Number(s.id) === Number(id));
  try {
    const res = await fetch('/api/v1/sites/' + id + '/export', { credentials: 'same-origin' });
    if (!res.ok) throw new Error('No se pudo exportar la sede');
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'netdiag-sede-' + (site ? site.slug : id) + '.json';
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 2000);
    toast('Paquete de sede descargado', { type: 'ok' });
  } catch (err) {
    toast(err.message || String(err), { type: 'err', title: 'Error' });
  }
}

function openImportSiteModal(id) {
  const site = (state.sites || []).find((s) => Number(s.id) === Number(id));
  const body = `
    <form data-form="site-import" data-id="${id}">
      <div class="field"><label>Paquete JSON de sede</label>
        <input class="input" type="file" name="file" accept="application/json,.json" required></div>
      <div class="field mt"><label>Modo</label>
        <select class="select" name="mode">
          <option value="merge">Combinar — conservar datos existentes</option>
          <option value="replace">Reemplazar — borrar datos actuales de la sede</option>
        </select></div>
      <div class="notice mt"><span>Se importarán redes, dispositivos y sus etiquetas/notas. El historial de escaneos y las métricas no se incluyen.</span></div>
    </form>`;
  openModal('Importar en ' + (site ? site.name : 'sede'), body, {
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="site-import-run" data-id="${id}">${icon('download')} Importar</button>`,
  });
}

async function submitSiteImport(form) {
  const id = Number(form.dataset.id || 0);
  const fd = new FormData(form);
  const file = fd.get('file');
  if (!file || !file.name) throw new Error('Selecciona un paquete JSON');
  const mode = String(fd.get('mode') || 'merge');
  const res = await fetch('/api/v1/sites/' + id + '/import?mode=' + encodeURIComponent(mode), {
    method: 'POST',
    credentials: 'same-origin',
    headers: state.csrf ? { 'X-CSRF-Token': state.csrf } : {},
    body: file,
  });
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch {}
  if (!res.ok) throw new Error((data && data.error) || 'No se pudo importar el paquete');
  const s = (data && data.summary) || {};
  closeModal();
  toast(`Importados ${s.networks_created || 0} redes y ${s.hosts_created || 0} dispositivos`, { type: 'ok' });
  await loadSites(true);
  await renderSettings();
}

async function settingsNetworksBody() {
  const [netsRes, sugRes] = await Promise.all([api('/networks'), api('/networks/suggest')]);
  state.networks = netsRes.items || [];
  const suggested = (sugRes.items || []).filter((s) => !s.already_added);
  const rows = state.networks.length ? state.networks.map((n) => `
    <tr>
      <td><div class="name-cell"><b>${esc(n.name || n.cidr)}</b><span class="mono">${esc(n.cidr)}</span></div></td>
      <td class="tiny muted">${esc(n.interface || '—')}</td>
      <td>${n.enabled ? '<span class="badge up">Habilitada</span>' : '<span class="badge unknown">Pausada</span>'}</td>
      <td class="tiny muted">${n.last_scan_at ? ago(n.last_scan_at) : 'Nunca'}</td>
      <td class="right nowrap">
        <button class="btn sm" data-action="network-edit" data-id="${n.id}">${icon('edit')} Editar</button>
        <button class="btn sm danger" data-action="network-delete" data-id="${n.id}">${icon('trash')}</button>
      </td>
    </tr>`).join('') : `<tr><td colspan="5"><div class="empty">${icon('network')}Sin redes configuradas</div></td></tr>`;
  const sugHTML = suggested.length ? `
    <div class="card mt">
      <div class="card-head"><h3>Redes detectadas en este equipo</h3>
        <button class="btn sm" data-action="network-suggest" data-refresh="1">${icon('refresh')} Volver a detectar</button></div>
      <div class="list">${suggested.map((s) => `
        <div class="list-item">
          <div class="type-icon">${icon('network')}</div>
          <div class="main"><div class="title mono">${esc(s.cidr)}</div>
            <div class="meta">${esc(s.interface || 'sin interfaz')} ${s.gateway ? '· puerta ' + esc(s.gateway) : ''}</div></div>
          <button class="btn sm primary" data-action="network-suggest" data-cidr="${esc(s.cidr)}" data-iface="${esc(s.interface || '')}">${icon('plus')} Añadir</button>
        </div>`).join('')}</div>
    </div>` : '';
  return `
    <div class="spread mb">
      <div class="muted small">Cada red define el alcance del monitor y del inventario. Las redes privadas se detectan automáticamente.</div>
      <button class="btn primary" data-action="network-new">${icon('plus')} Nueva red</button>
    </div>
    <div class="card"><div class="table-wrap"><table>
      <thead><tr><th>Red</th><th>Interfaz</th><th>Estado</th><th>Último escaneo</th><th></th></tr></thead>
      <tbody>${rows}</tbody>
    </table></div></div>
    ${sugHTML}`;
}

async function settingsUsersBody() {
  const [usersRes, status] = await Promise.all([api('/auth/users'), api('/auth/status')]);
  const users = usersRes.items || [];
  const me = (status && status.user) || state.me || {};
  const roleOptions = (u) => [['admin', 'Administrador'], ['operator', 'Operador'], ['viewer', 'Solo lectura']]
    .map(([v, l]) => `<option value="${v}" ${u.role === v ? 'selected' : ''}>${esc(l)}</option>`).join('');
  const rows = users.map((u) => {
    const self = Number(u.id) === Number(me.id);
    return `
    <tr>
      <td><div class="row"><div class="avatar">${esc((u.username || '?').slice(0, 1).toUpperCase())}</div>
        <div class="name-cell"><b>${esc(u.username)}</b><span>${esc(roleName(u.role))}${self ? ' · tú' : ''}</span></div></div></td>
      <td class="tiny muted">${fmtDate(u.created_at)}</td>
      <td class="tiny muted">${u.last_login_at ? ago(u.last_login_at) : 'Nunca'}</td>
      <td><select class="select sm" data-user-role="${u.id}" ${self ? 'disabled title="No puedes cambiar tu propio rol"' : ''}>${roleOptions(u)}</select></td>
      <td class="right nowrap">
        <button class="btn sm" data-action="user-reset" data-id="${u.id}" data-name="${esc(u.username)}">${icon('edit')} Contraseña</button>
        <button class="btn sm danger" data-action="user-delete" data-id="${u.id}" data-name="${esc(u.username)}" data-self="${self ? '1' : '0'}" ${self ? 'disabled' : ''}>${icon('trash')}</button>
      </td>
    </tr>`;
  }).join('');
  return `
    <div class="spread mb">
      <div class="muted small">${status.auth_enabled ? 'La autenticación está activa. Los visores pueden consultar pero no modificar.' : 'La autenticación está desactivada: cualquiera con acceso al puerto es administrador.'}</div>
      <button class="btn primary" data-action="user-new">${icon('plus')} Nuevo usuario</button>
    </div>
    <div class="card"><div class="table-wrap"><table>
      <thead><tr><th>Usuario</th><th>Creado</th><th>Último acceso</th><th>Rol</th><th></th></tr></thead>
      <tbody>${rows || '<tr><td colspan="5"><div class="empty">Sin usuarios</div></td></tr>'}</tbody>
    </table></div></div>
    <div class="notice mt"><span><b>Roles:</b> administrador controla todo; operador puede escanear, editar inventario y gestionar alertas; solo lectura no puede modificar nada.</span></div>`;
}

async function settingsBackupsBody() {
  const res = await api('/system/backups');
  const items = res.items || [];
  const rows = items.map((b) => `
    <tr>
      <td class="mono">${esc(b.name)}</td>
      <td>${fmtBytes(b.size)}</td>
      <td class="tiny muted">${fmtDate(b.created_at)}</td>
      <td class="right nowrap">
        <a class="btn sm" href="/api/v1/system/backups/${encodeURIComponent(b.name)}/download">${icon('download')} Descargar</a>
        <button class="btn sm danger" data-action="backup-delete" data-name="${esc(b.name)}">${icon('trash')}</button>
      </td>
    </tr>`).join('');
  return `
    <div class="spread mb">
      <div class="muted small">La copia incluye la base de datos, config.yaml y secret.key. Guárdala en un lugar cifrado.</div>
      <button class="btn primary" data-action="backup-create">${icon('plus')} Crear copia ahora</button>
    </div>
    <div class="card pad mb">
      <div class="spread"><div><div class="label muted tiny">Directorio de copias</div>
        <div class="mono small">${esc(res.dir || '—')}</div></div>
        <span class="badge info">${items.length} copia${items.length === 1 ? '' : 's'}</span></div>
    </div>
    <div class="card"><div class="table-wrap"><table>
      <thead><tr><th>Archivo</th><th>Tamaño</th><th>Creada</th><th></th></tr></thead>
      <tbody>${rows || '<tr><td colspan="4"><div class="empty">Aún no hay copias</div></td></tr>'}</tbody>
    </table></div></div>`;
}

async function settingsSystemBody() {
  const [info, audit] = await Promise.all([api('/system/info'), api('/system/audit?limit=40')]);
  const caps = Object.entries(info.capabilities || {}).map(([k, v]) =>
    `<span class="badge ${v ? 'up' : 'unknown'}">${esc(k)}</span>`).join(' ');
  const ifaces = (info.interfaces || []).map((i) => `
    <tr>
      <td class="mono">${esc(i.name)}</td>
      <td>${i.up ? '<span class="badge up">Up</span>' : '<span class="badge down">Down</span>'}</td>
      <td class="tiny mono">${(i.addrs || []).map((a) => esc(a.cidr)).join('<br>') || '—'}</td>
      <td class="tiny mono">${esc(i.mac || '—')}</td>
      <td class="tiny mono">${esc(i.gateway || '—')}</td>
    </tr>`).join('');
  const auditRows = (audit.items || []).map((a) => `
    <tr>
      <td class="tiny muted nowrap">${fmtDate(a.ts)}</td>
      <td class="tiny">${esc(a.actor)}</td>
      <td class="tiny mono">${esc(a.action)}</td>
      <td class="tiny">${esc(a.target || '')}</td>
      <td class="tiny muted">${esc(a.ip || '')}</td>
    </tr>`).join('');
  return `
    <div class="grid cols-3 mb">
      <div class="card pad"><div class="label muted tiny" style="letter-spacing:.08em;text-transform:uppercase">Versión</div>
        <div style="font-size:19px;font-weight:700;margin-top:4px">${esc(info.version)}</div>
        <div class="tiny muted">commit ${esc((info.commit || 'dev').slice(0, 12))} · ${esc(info.build_date || 'sin fecha')}</div></div>
      <div class="card pad"><div class="label muted tiny" style="letter-spacing:.08em;text-transform:uppercase">Servidor</div>
        <div style="font-size:19px;font-weight:700;margin-top:4px">${esc(info.hostname)}</div>
        <div class="tiny muted">${esc(info.os)}/${esc(info.arch)} · ${esc(info.go_version)}</div></div>
      <div class="card pad"><div class="label muted tiny" style="letter-spacing:.08em;text-transform:uppercase">Datos</div>
        <div style="font-size:19px;font-weight:700;margin-top:4px">${fmtBytes(info.db_size)}</div>
        <div class="tiny muted mono ellipsis" title="${esc(info.data_dir)}">${esc(info.data_dir)}</div></div>
    </div>
    <div class="card pad mb">
      <div class="spread mb"><h3 style="margin:0;font-size:13.5px">Capacidades</h3>
        <span class="tiny muted">activo desde ${fmtDate(info.started_at)} (${Math.floor((info.uptime_secs || 0) / 3600)} h)</span></div>
      <div class="row wrap">${caps || '<span class="muted">—</span>'}</div>
    </div>
    <div class="card mb">
      <div class="card-head"><h3>Interfaces de red</h3></div>
      <div class="table-wrap"><table>
        <thead><tr><th>Interfaz</th><th>Estado</th><th>Direcciones</th><th>MAC</th><th>Puerta</th></tr></thead>
        <tbody>${ifaces || '<tr><td colspan="5"><div class="empty">Sin interfaces</div></td></tr>'}</tbody>
      </table></div>
    </div>
    <div class="card">
      <div class="card-head"><h3>Auditoría reciente</h3>
        <button class="btn sm" data-action="reload">${icon('refresh')} Actualizar</button></div>
      <div class="table-wrap"><table>
        <thead><tr><th>Fecha</th><th>Actor</th><th>Acción</th><th>Objetivo</th><th>IP</th></tr></thead>
        <tbody>${auditRows || '<tr><td colspan="5"><div class="empty">Sin actividad registrada</div></td></tr>'}</tbody>
      </table></div>
    </div>`;
}

function openNetworkModal(id) {
  const net = id ? state.networks.find((n) => n.id === id) : null;
  const body = `
    <form data-form="network" data-id="${net ? net.id : ''}">
      <div class="form-grid">
        <div class="field full"><label>Nombre</label>
          <input class="input" name="name" maxlength="80" value="${esc(net ? net.name : '')}" placeholder="Ej.: Oficina principal"></div>
        <div class="field"><label>CIDR</label>
          <input class="input mono" name="cidr" required value="${esc(net ? net.cidr : '')}" placeholder="192.168.1.0/24"></div>
        <div class="field"><label>Interfaz (opcional)</label>
          <input class="input mono" name="interface" value="${esc(net ? net.interface : '')}" placeholder="eth0"></div>
        <div class="field full"><label>Estado</label>
          <label class="check"><input type="checkbox" name="enabled" ${!net || net.enabled ? 'checked' : ''}> Red habilitada para monitorización</label></div>
      </div>
    </form>`;
  openModal(net ? 'Editar red' : 'Nueva red', body, {
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="network-save" data-id="${net ? net.id : ''}">${icon('check')} Guardar</button>`,
  });
}

async function submitNetwork(form) {
  const fd = new FormData(form);
  const id = Number(form.dataset.id || 0);
  const payload = {
    name: String(fd.get('name') || '').trim(),
    cidr: String(fd.get('cidr') || '').trim(),
    interface: String(fd.get('interface') || '').trim(),
    enabled: fd.get('enabled') === 'on',
  };
  if (!id) payload.site_id = Number(state.siteId) || 1;
  if (!payload.cidr) throw new Error('El CIDR es obligatorio');
  if (id) await api('/networks/' + id, { method: 'PUT', body: payload });
  else await api('/networks', { method: 'POST', body: payload });
  closeModal();
  toast(id ? 'Red actualizada' : 'Red añadida', { type: 'ok' });
  state.networks = [];
  await renderSettings();
}

async function deleteNetwork(id) {
  if (!confirm('¿Eliminar esta red? Los dispositivos permanecen en el inventario, pero dejan de monitorizarse.')) return;
  await api('/networks/' + id, { method: 'DELETE' });
  toast('Red eliminada', { type: 'ok' });
  state.networks = [];
  await renderSettings();
}

async function addSuggestedNetwork(cidr, iface) {
  if (!cidr) { await renderSettings(); return; }
  await api('/networks', { method: 'POST', body: { name: cidr, cidr, interface: iface || '', site_id: Number(state.siteId) || 1 } });
  toast('Red añadida: ' + cidr, { type: 'ok' });
  state.networks = [];
  await renderSettings();
}

function openUserModal() {
  const body = `
    <form data-form="user">
      <div class="form-grid">
        <div class="field"><label>Usuario</label>
          <input class="input" name="username" required maxlength="64" autocomplete="off" placeholder="operador"></div>
        <div class="field"><label>Rol</label>
          <select class="select" name="role"><option value="viewer">Solo lectura</option><option value="operator">Operador</option><option value="admin">Administrador</option></select></div>
        <div class="field full"><label>Contraseña</label>
          <input class="input" name="password" type="password" required minlength="10" autocomplete="new-password">
          <span class="help">Mínimo 10 caracteres, con letras y números.</span></div>
      </div>
    </form>`;
  openModal('Nuevo usuario', body, {
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="user-save">${icon('check')} Crear usuario</button>`,
  });
}

async function submitUser(form) {
  const fd = new FormData(form);
  const user = await api('/auth/users', {
    method: 'POST',
    body: {
      username: String(fd.get('username') || '').trim(),
      password: String(fd.get('password') || ''),
      role: String(fd.get('role') || 'viewer'),
    },
  });
  closeModal();
  toast(`Usuario ${user.username} creado`, { type: 'ok' });
  await renderSettings();
}

async function changeUserRole(id, role) {
  await api('/auth/users/' + id, { method: 'PATCH', body: { role } });
  toast('Rol actualizado', { type: 'ok' });
  await renderSettings();
  if (state.me && Number(state.me.id) === id) {
    state.me.role = role;
    renderShell();
    await route();
  }
}

function openResetUserModal(id, username) {
  const body = `
    <form data-form="reset-password" data-id="${id}">
      <div class="field"><label>Nueva contraseña para ${esc(username)}</label>
        <input class="input" name="password" type="password" required minlength="10" autocomplete="new-password">
        <span class="help">Mínimo 10 caracteres, con letras y números. Se cerrarán todas sus sesiones.</span></div>
    </form>`;
  openModal('Restablecer contraseña', body, {
    footer: `<button class="btn" data-action="modal-close">Cancelar</button>
      <button class="btn primary" data-action="user-password-save">${icon('check')} Restablecer</button>`,
  });
}

async function submitUserPasswordFromModal() {
  const form = document.querySelector('form[data-form="reset-password"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="user-password-save"]');
  if (btn) btn.disabled = true;
  try {
    await submitUserPassword(form);
  } catch (e) {
    if (btn) btn.disabled = false;
    throw e;
  }
}

async function submitUserPassword(form) {
  const id = Number(form.dataset.id || 0);
  const fd = new FormData(form);
  await api('/auth/users/' + id + '/password', {
    method: 'POST',
    body: { password: String(fd.get('password') || '') },
  });
  closeModal();
  toast('Contraseña restablecida', { type: 'ok' });
}

async function deleteUserAccount(id, name, self) {
  if (self) { toast('No puedes eliminar tu propia cuenta', { type: 'warn' }); return; }
  if (!confirm(`¿Eliminar al usuario ${name}? Se cerrarán todas sus sesiones.`)) return;
  await api('/auth/users/' + id, { method: 'DELETE' });
  toast('Usuario eliminado', { type: 'ok' });
  await renderSettings();
}

async function createBackup(btn) {
  if (btn) { btn.disabled = true; btn.innerHTML = 'Creando…'; }
  try {
    const entry = await api('/system/backups', { method: 'POST' });
    toast(`Copia creada: ${entry.name} (${fmtBytes(entry.size)})`, { type: 'ok' });
    await renderSettings();
  } finally {
    if (btn) { btn.disabled = false; }
  }
}

async function deleteBackup(name) {
  if (!confirm(`¿Eliminar la copia ${name}?`)) return;
  await api('/system/backups/' + encodeURIComponent(name), { method: 'DELETE' });
  toast('Copia eliminada', { type: 'ok' });
  await renderSettings();
}

/* Actions used by buttons inside modals. */
async function saveRuleFromModal(id) {
  const form = document.querySelector('form[data-form="rule"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="rule-save"]');
  if (btn) btn.disabled = true;
  try {
    await submitRule(form);
  } catch (e) {
    if (btn) btn.disabled = false;
    throw e;
  }
}

async function saveChannelFromModal(id) {
  const form = document.querySelector('form[data-form="channel"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="channel-save"]');
  if (btn) btn.disabled = true;
  try {
    await submitChannel(form);
  } catch (e) {
    if (btn) btn.disabled = false;
    throw e;
  }
}

async function saveNetworkFromModal(id) {
  const form = document.querySelector('form[data-form="network"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="network-save"]');
  if (btn) btn.disabled = true;
  try {
    await submitNetwork(form);
  } catch (e) {
    if (btn) btn.disabled = false;
    throw e;
  }
}

async function saveUserFromModal() {
  const form = document.querySelector('form[data-form="user"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="user-save"]');
  if (btn) btn.disabled = true;
  try {
    await submitUser(form);
  } catch (e) {
    if (btn) btn.disabled = false;
    throw e;
  }
}

async function createReportFromModal() {
  const form = document.querySelector('form[data-form="report"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="report-create"]');
  if (btn) { btn.disabled = true; btn.innerHTML = 'Generando…'; }
  try {
    await submitReport(form);
  } catch (e) {
    if (btn) { btn.disabled = false; btn.innerHTML = 'Generar'; }
    throw e;
  }
}

async function startScanFromModal() {
  const form = document.querySelector('form[data-form="scan"]');
  if (!form) return;
  const btn = document.querySelector('[data-action="scan-start"]');
  if (btn) { btn.disabled = true; btn.innerHTML = 'Iniciando…'; }
  try {
    await submitScan(form);
  } catch (e) {
    if (btn) { btn.disabled = false; btn.innerHTML = 'Iniciar'; }
    throw e;
  }
}

async function toggleChannel(id, enabled) {
  const ch = state.channels.find((c) => c.id === id);
  if (!ch) return;
  await api('/alerts/channels/' + id, {
    method: 'PUT',
    body: { name: ch.name, type: ch.type, enabled, config: ch.config || {} },
  });
  toast(enabled ? 'Canal habilitado' : 'Canal deshabilitado', { type: 'ok' });
  state.channels = [];
  await renderAlerts('canales');
}

/* ------------------------------------------------------------------ */
/* Guía interactiva                                                    */
/* ------------------------------------------------------------------ */

const GUIDE_SECTIONS = [
  {
    id: 'inicio', icon: 'play', title: 'Primeros pasos', titleEn: 'Getting started',
    intro: 'De cero a una red monitorizada en cinco pasos.',
    introEn: 'From zero to a monitored network in five steps.',
    items: [
      {
        t: '1. Añade tu primera red', tEn: '1. Add your first network',
        b: 'En Configuración → Redes pulsa «Nueva red» o acepta una de las redes detectadas automáticamente. Usa el CIDR real (por ejemplo 192.168.1.0/24) y, si quieres, asócialo a una interfaz concreta.',
        bEn: 'In Settings → Networks click “New network”, or accept one of the automatically detected networks. Use the real CIDR (for example 192.168.1.0/24) and, if you want, bind it to a specific interface.',
      },
      {
        t: '2. Lanza un escaneo', tEn: '2. Run a scan',
        b: 'Pulsa «Escanear», elige la red y el perfil. «Común» cubre la mayoría de casos; «Completo» añade más puertos y enriquecimiento SNMP. Puedes seguir el progreso en Escaneos o en la cabecera.',
        bEn: 'Click “Scan”, pick the network and profile. “Common” covers most cases; “Full” adds more ports and SNMP enrichment. Follow progress in Scans or in the header.',
      },
      {
        t: '3. Revisa el inventario', tEn: '3. Review the inventory',
        b: 'En Dispositivos verás IP, MAC, fabricante, tipo probable, sistema operativo, puertos y servicios. Abre un host para ver historial, vecinos y certificados.',
        bEn: 'In Devices you see IP, MAC, vendor, likely type, operating system, ports and services. Open a host to see history, neighbors and certificates.',
      },
      {
        t: '4. Activa el monitor', tEn: '4. Turn on the monitor',
        b: 'En Configuración → Escaneo activa el monitor periódico. NetDiag escaneará las redes habilitadas, detectará hosts nuevos y caídos y guardará latencia y disponibilidad.',
        bEn: 'In Settings → Scanning enable the periodic monitor. NetDiag will scan the enabled networks, detect new and down hosts, and store latency and availability.',
      },
      {
        t: '5. Configura alertas', tEn: '5. Configure alerts',
        b: 'En Alertas revisa las reglas integradas y crea las tuyas. Añade un canal (webhook, Slack, Telegram, correo o Syslog) y envíale una prueba antes de confiar en él.',
        bEn: 'In Alerts review the built-in rules and create your own. Add a channel (webhook, Slack, Telegram, email or Syslog) and send it a test before relying on it.',
      },
    ],
  },
  {
    id: 'conceptos', icon: 'book', title: 'Conceptos', titleEn: 'Concepts',
    intro: 'El modelo mental que hace que todo lo demás encaje.',
    introEn: 'The mental model that makes everything else click.',
    items: [
      {
        t: 'Host e identidad estable', tEn: 'Hosts and stable identity',
        b: 'Un host es un dispositivo descubierto. Su identidad se rekeyea a la MAC cuando se conoce, de modo que notas, etiquetas, favoritos e historial sobreviven a cambios de IP.',
        bEn: 'A host is a discovered device. Its identity is rekeyed to the MAC when known, so notes, tags, favorites and history survive IP changes.',
      },
      {
        t: 'Perfiles de escaneo', tEn: 'Scan profiles',
        b: 'Rápido: descubrimiento y pocos puertos. Común: puertos habituales y servicios. Completo: rango amplio, banners, TLS y SNMP. Personalizado: tu lista de puertos.',
        bEn: 'Quick: discovery and a few ports. Common: usual ports and services. Full: wide range, banners, TLS and SNMP. Custom: your own port list.',
      },
      {
        t: 'Confianza en topología', tEn: 'Topology confidence',
        b: 'Los enlaces proceden de LLDP/CDP (alta confianza), tablas de bridge/FDB (media) y ARP (baja). El mapa muestra el origen para que sepas cuánto fiarse de cada arista.',
        bEn: 'Links come from LLDP/CDP (high confidence), bridge/FDB tables (medium) and ARP (low). The map shows the source so you know how much to trust each edge.',
      },
      {
        t: 'Ciclo de una alerta', tEn: 'Alert lifecycle',
        b: 'Activa → reconocida → resuelta. Reconocer indica que alguien la está mirando; resolver la cierra. Las reglas integradas se pueden desactivar sin borrarlas.',
        bEn: 'Active → acknowledged → resolved. Acknowledging shows someone is on it; resolving closes it. Built-in rules can be disabled without deleting them.',
      },
      {
        t: 'Roles', tEn: 'Roles',
        b: 'Administrador: todo, incluida configuración, usuarios y copias. Operador: escanea, edita inventario y gestiona alertas. Solo lectura: consulta sin modificar nada.',
        bEn: 'Administrator: everything, including settings, users and backups. Operator: scans, edits inventory and manages alerts. Read-only: can view but change nothing.',
      },
      {
        t: 'Datos y secretos', tEn: 'Data and secrets',
        b: 'Todo vive en un único directorio de datos: SQLite, informes, logs y secret.key. Las credenciales de canales y SNMPv3 se cifran con AES-256-GCM.',
        bEn: 'Everything lives in a single data directory: SQLite, reports, logs and secret.key. Channel and SNMPv3 credentials are encrypted with AES-256-GCM.',
      },
    ],
  },
  {
    id: 'flujos', icon: 'topology', title: 'Flujos de trabajo', titleEn: 'Workflows',
    intro: 'Recetas para el día a día.',
    introEn: 'Recipes for everyday use.',
    items: [
      {
        t: 'Revisión diaria', tEn: 'Daily review',
        b: 'Panel → Alertas activas → Dispositivos con estado caído. Usa el filtro «Solo favoritos» para vigilar la infraestructura crítica.',
        bEn: 'Dashboard → Active alerts → Down devices. Use the “Favorites only” filter to watch critical infrastructure.',
      },
      {
        t: 'Investigar un dispositivo nuevo', tEn: 'Investigate a new device',
        b: 'Recibe el aviso «Nuevo dispositivo», búscalo por IP o MAC y revisa fabricante, puertos y vecinos. Añade notas y etiquetas si es infraestructura.',
        bEn: 'Get the “New host” alert, find it by IP or MAC, and check vendor, ports and neighbors. Add notes and tags if it is infrastructure.',
      },
      {
        t: 'Responder a una alerta', tEn: 'Respond to an alert',
        b: 'Abre la alerta, revisa el host afectado y su historial. Reconócela mientras investigas y resuélvela al terminar, o silencia la regla si es ruido.',
        bEn: 'Open the alert, review the affected host and its history. Acknowledge while you investigate, resolve when done, or disable the rule if it is noise.',
      },
      {
        t: 'Preparar un informe', tEn: 'Prepare a report',
        b: 'Informes → Generar informe. «Inventario» para auditoría, «Ejecutivo» para dirección, «Alertas» para operación. PDF para compartir, CSV/JSON para análisis.',
        bEn: 'Reports → Generate report. “Inventory” for audits, “Executive” for management, “Alerts” for operations. PDF to share, CSV/JSON for analysis.',
      },
      {
        t: 'Copiar y restaurar', tEn: 'Back up and restore',
        b: 'Configuración → Copias crea un ZIP con base de datos, config.yaml y secret.key. Para restaurar: detén el servicio, sustituye esos tres archivos, ajusta permisos y arranca.',
        bEn: 'Settings → Backups creates a ZIP with the database, config.yaml and secret.key. To restore: stop the service, replace those three files, fix permissions and start it again.',
      },
    ],
  },
  {
    id: 'seguridad', icon: 'shield', title: 'Seguridad', titleEn: 'Security',
    intro: 'NetDiag está pensado para quedarse dentro de tu red.',
    introEn: 'NetDiag is meant to stay inside your network.',
    items: [
      {
        t: 'Autenticación y sesiones', tEn: 'Authentication and sessions',
        b: 'Argon2id para contraseñas, sesiones opacas en servidor y cierre global al cambiar o restablecer una contraseña. Configura security.session_hours según tu política.',
        bEn: 'Argon2id for passwords, opaque server-side sessions, and a global logout when a password is changed or reset. Set security.session_hours to match your policy.',
      },
      {
        t: 'CSRF y cabeceras', tEn: 'CSRF and headers',
        b: 'Toda mutación exige X-CSRF-Token. La API envía CSP, HSTS opcional, X-Frame-Options, nosniff y Referrer-Policy.',
        bEn: 'Every mutation requires X-CSRF-Token. The API sends CSP, optional HSTS, X-Frame-Options, nosniff and Referrer-Policy.',
      },
      {
        t: 'Hosts permitidos', tEn: 'Allowed hosts',
        b: 'Si publicas NetDiag tras un proxy, define security.allowed_hosts para bloquear DNS rebinding. Una petición con Host no permitido recibe 421.',
        bEn: 'If you publish NetDiag behind a proxy, set security.allowed_hosts to block DNS rebinding. A request with a disallowed Host gets 421.',
      },
      {
        t: 'Métricas y OpenAPI', tEn: 'Metrics and OpenAPI',
        b: '/metrics y /api/v1/openapi.json son públicos por defecto. Si defines security.metrics_token, Prometheus debe enviar Authorization: Bearer <token>.',
        bEn: '/metrics and /api/v1/openapi.json are public by default. If you set security.metrics_token, Prometheus must send Authorization: Bearer <token>.',
      },
      {
        t: 'Secretos', tEn: 'Secrets',
        b: 'Los canales y SNMPv3 se cifran en reposo. La UI nunca devuelve el secreto: muestra ******** y conserva el valor si dejas el campo vacío.',
        bEn: 'Channels and SNMPv3 are encrypted at rest. The UI never returns secrets: it shows ******** and keeps the stored value when you leave the field empty.',
      },
      {
        t: 'Menor privilegio', tEn: 'Least privilege',
        b: 'Ejecuta el servicio como usuario sin login, con las capabilities mínimas del unit systemd. No lo expongas directamente a Internet sin TLS y proxy.',
        bEn: 'Run the service as a no-login user with the minimum capabilities in the systemd unit. Never expose it directly to the Internet without TLS and a proxy.',
      },
    ],
  },
  {
    id: 'trucos', icon: 'bolt', title: 'Atajos y trucos', titleEn: 'Tips and shortcuts',
    intro: 'Detalles que ahorran tiempo.',
    introEn: 'Small details that save time.',
    items: [
      {
        t: 'Búsqueda global del inventario', tEn: 'Global inventory search',
        b: 'El buscador de Dispositivos acepta IP, hostname, MAC y fabricante. Combínalo con estado, tipo, red y favoritos.',
        bEn: 'The Devices search accepts IP, hostname, MAC and vendor. Combine it with status, type, network and favorites.',
      },
      {
        t: 'Idioma y tema', tEn: 'Language and theme',
        b: 'El botón ES/EN de la cabecera cambia el idioma y lo guarda en el navegador. El icono de tema cicla entre oscuro, claro y alto contraste; sin preferencia guardada respeta prefers-color-scheme.',
        bEn: 'The ES/EN button in the header switches language and remembers it in the browser. The theme icon cycles between dark, light and high contrast; with no saved preference it respects prefers-color-scheme.',
      },
      {
        t: 'Esc para cerrar', tEn: 'Esc to close',
        b: 'Los modales se cierran con Escape o pulsando fuera. El recorrido guiado se puede saltar en cualquier momento.',
        bEn: 'Modals close with Escape or by clicking outside. The guided tour can be skipped at any time.',
      },
      {
        t: 'Enlaces directos', tEn: 'Direct links',
        b: 'Cada vista tiene hash propio: #/hosts, #/topology, #/alerts, #/scans, #/reports, #/guide y #/settings. Puedes guardarlos como marcadores.',
        bEn: 'Each view has its own hash: #/hosts, #/topology, #/alerts, #/scans, #/reports, #/guide and #/settings. Bookmark them freely.',
      },
      {
        t: 'Datos en vivo', tEn: 'Live data',
        b: 'La barra inferior indica «En vivo» cuando el WebSocket está conectado. Si cae, se reconecta solo y las vistas se refrescan al volver.',
        bEn: 'The footer shows “Live” while the WebSocket is connected. If it drops it reconnects on its own and views refresh when it returns.',
      },
    ],
  },
  {
    id: 'problemas', icon: 'warn', title: 'Solución de problemas', titleEn: 'Troubleshooting',
    intro: 'Lo que suele fallar y cómo comprobarlo.',
    introEn: 'What usually breaks and how to check it.',
    items: [
      {
        t: 'No aparecen hosts', tEn: 'No hosts show up',
        b: 'Comprueba que la red está habilitada, que el CIDR es correcto y que el proceso tiene permiso para ARP/raw sockets. Ejecuta netdiag doctor y revisa los logs.',
        bEn: 'Check that the network is enabled, the CIDR is right, and the process may use ARP/raw sockets. Run netdiag doctor and review the logs.',
      },
      {
        t: 'El escaneo SNMP no devuelve nada', tEn: 'SNMP scanning returns nothing',
        b: 'Verifica comunidad/versión, UDP 161 y ACL del dispositivo. Para v3 revisa usuario, protocolo de autenticación y privacidad.',
        bEn: 'Verify community/version, UDP 161 and the device ACL. For v3 check the user, authentication protocol and privacy protocol.',
      },
      {
        t: '421 host no permitido', tEn: '421 host not allowed',
        b: 'La petición llega con un Host distinto de security.allowed_hosts. Ajusta la lista o el proxy; la SPA se sirve igual, pero la API responde 421.',
        bEn: 'The request arrives with a Host outside security.allowed_hosts. Fix the list or the proxy; the SPA still loads, but the API answers 421.',
      },
      {
        t: '/metrics devuelve 401', tEn: '/metrics returns 401',
        b: 'Hay security.metrics_token configurado. Añade authorization: Bearer <token> en el scrape de Prometheus o vacía el campo para desactivarlo.',
        bEn: 'security.metrics_token is set. Add authorization: Bearer <token> to the Prometheus scrape, or clear the field to disable it.',
      },
      {
        t: 'Un visor no puede editar', tEn: 'A viewer cannot edit',
        b: 'Es el comportamiento esperado. Cambia su rol a operador o administrador desde Configuración → Usuarios.',
        bEn: 'That is expected. Change the role to operator or administrator in Settings → Users.',
      },
      {
        t: 'Necesito recuperar una copia', tEn: 'I need to restore a backup',
        b: 'Descarga el ZIP, detén NetDiag, extrae netdiag.db, config.yaml y secret.key en el directorio de datos, conserva permisos 600 y arranca de nuevo.',
        bEn: 'Download the ZIP, stop NetDiag, extract netdiag.db, config.yaml and secret.key into the data directory, keep mode 600 and start again.',
      },
    ],
  },
];

function guideText(obj, key) {
  const en = window.NetDiagI18n && window.NetDiagI18n.isEnglish;
  if (en && obj[key + 'En']) return obj[key + 'En'];
  return obj[key];
}

function guideSectionHTML(sec) {
  return `<section class="guide-section card" id="guide-${sec.id}" data-guide-text>
    <div class="card-head"><h3>${icon(sec.icon)} ${esc(guideText(sec, 'title'))}</h3><span class="tiny muted">${esc(guideText(sec, 'intro'))}</span></div>
    <div class="guide-items">
      ${sec.items.map((item, i) => `
        <details class="guide-item" ${i === 0 ? 'open' : ''}>
          <summary>${esc(guideText(item, 't'))}</summary>
          <p>${esc(guideText(item, 'b'))}</p>
        </details>`).join('')}
    </div>
  </section>`;
}

async function renderGuide() {
  setPageTitle('guide');
  const en = window.NetDiagI18n && window.NetDiagI18n.isEnglish;
  const pick = (es, english) => (en ? english : es);
  const content = document.getElementById('content');
  content.innerHTML = `
    <div class="guide-hero card pad mb">
      <div class="spread wrap">
        <div>
          <h2 style="margin:0 0 6px">${esc(pick('Guía de uso', 'User guide'))}</h2>
          <p class="muted" style="margin:0;max-width:620px">${esc(pick(
            'Todo lo que necesitas para operar NetDiag, sin salir del producto. La guía es interactiva: puedes buscar, desplegar cada tema y repetir el recorrido guiado.',
            'Everything you need to operate NetDiag, without leaving the product. Search, expand each topic and replay the guided tour.'
          ))}</p>
        </div>
        <button class="btn primary" data-action="tour-start">${icon('play')} ${esc(pick('Iniciar recorrido guiado', 'Start guided tour'))}</button>
      </div>
      <div class="guide-search mt">
        <input class="input" id="guide-search" data-guide-search placeholder="${esc(pick('Buscar en la guía…', 'Search the guide…'))}" autocomplete="off">
      </div>
      <div class="row wrap mt">
        ${GUIDE_SECTIONS.map((s) => `<a class="btn sm" href="#guide-${s.id}">${icon(s.icon)} ${esc(guideText(s, 'title'))}</a>`).join('')}
      </div>
    </div>
    <div class="guide-grid">
      ${GUIDE_SECTIONS.map(guideSectionHTML).join('')}
    </div>
    <div class="card pad mt">
      <div class="spread wrap">
        <div><h3 style="margin:0 0 4px">${esc(pick('Referencia técnica', 'Technical reference'))}</h3>
          <div class="muted small">${esc(pick('La especificación OpenAPI se publica en', 'The OpenAPI specification is published at'))} <span class="mono">/api/v1/openapi.json</span>; ${esc(pick('las métricas Prometheus en', 'Prometheus metrics at'))} <span class="mono">/metrics</span>.</div></div>
        <div class="row wrap">
          <a class="btn sm" href="/api/v1/openapi.json" target="_blank" rel="noopener">${icon('reports')} OpenAPI</a>
          <a class="btn sm" href="/metrics" target="_blank" rel="noopener">${icon('bolt')} /metrics</a>
        </div>
      </div>
    </div>`;
}

function filterGuide(query) {
  const q = String(query || '').trim().toLowerCase();
  document.querySelectorAll('[data-guide-text]').forEach((sec) => {
    let visible = false;
    sec.querySelectorAll('.guide-item').forEach((item) => {
      const match = !q || item.textContent.toLowerCase().includes(q);
      item.classList.toggle('hidden', !match);
      if (match) visible = true;
    });
    sec.classList.toggle('hidden', !visible);
  });
}
