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
  ['general', 'General'],
  ['escaneo', 'Escaneo'],
  ['alertas', 'Alertas'],
  ['seguridad', 'Seguridad'],
  ['almacenamiento', 'Almacenamiento'],
  ['redes', 'Redes'],
  ['usuarios', 'Usuarios'],
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
    st = cfg.storage || {}, sec = cfg.security || {}, log = cfg.log || {};
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
  } else if (tab === 'seguridad') {
    fields = `<div class="form-grid">
      <div class="field"><label>Autenticación</label>
        <label class="check"><input type="checkbox" name="security.auth" ${sec.auth ? 'checked' : ''}> Exigir inicio de sesión</label>
        <span class="help">Si se desactiva, la instancia queda abierta a cualquiera que alcance el puerto.</span></div>
      ${cfgInput('Duración de sesión (horas)', 'security.session_hours', sec.session_hours, { type: 'number', min: 1, max: 720 })}
      ${cfgInput('Intentos de login / minuto', 'security.login_rate', sec.login_rate, { type: 'number', min: 1, max: 100 })}
      ${cfgInput('Peticiones API / minuto', 'security.api_rate', sec.api_rate, { type: 'number', min: 10, max: 100000 })}
      ${cfgInput('Hosts permitidos', 'security.allowed_hosts', (sec.allowed_hosts || []).join(', '), { placeholder: 'netdiag.example.com', help: 'Separados por comas; vacío = cualquiera.' })}
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
    </div>`;
  }
  const wrapForm = ['general', 'escaneo', 'alertas', 'seguridad', 'almacenamiento'].includes(tab);
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
      'alerts.max_per_minute', 'security.session_hours', 'security.login_rate', 'security.api_rate',
      'storage.retention_days', 'storage.metrics_days', 'storage.audit_days', 'storage.max_scans'].includes(name)) {
      setPath(next, name, Number(value));
      return;
    }
    if (name === 'scan.snmp_v3_auth' || name === 'scan.snmp_v3_priv') {
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
  if (tab === 'redes') {
    body = await settingsNetworksBody();
  } else if (tab === 'usuarios') {
    body = await settingsUsersBody();
  } else if (tab === 'sistema') {
    body = await settingsSystemBody();
  } else {
    const res = await api('/system/config');
    state.settingsCfg = res.config;
    body = configFormBody(res.config);
  }
  content.innerHTML = tabsHTML + body;
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
  const rows = users.map((u) => `
    <tr>
      <td><div class="row"><div class="avatar">${esc((u.username || '?').slice(0, 1).toUpperCase())}</div>
        <div class="name-cell"><b>${esc(u.username)}</b><span>${esc(u.role)}</span></div></div></td>
      <td class="tiny muted">${fmtDate(u.created_at)}</td>
      <td class="tiny muted">${u.last_login_at ? ago(u.last_login_at) : 'Nunca'}</td>
      <td>${u.role === 'admin' ? '<span class="badge info">Administrador</span>' : '<span class="badge unknown">Solo lectura</span>'}</td>
    </tr>`).join('');
  return `
    <div class="spread mb">
      <div class="muted small">${status.auth_enabled ? 'La autenticación está activa. Los visores pueden consultar pero no modificar.' : 'La autenticación está desactivada: cualquiera con acceso al puerto es administrador.'}</div>
      <button class="btn primary" data-action="user-new">${icon('plus')} Nuevo usuario</button>
    </div>
    <div class="card"><div class="table-wrap"><table>
      <thead><tr><th>Usuario</th><th>Creado</th><th>Último acceso</th><th>Rol</th></tr></thead>
      <tbody>${rows || '<tr><td colspan="4"><div class="empty">Sin usuarios</div></td></tr>'}</tbody>
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
  await api('/networks', { method: 'POST', body: { name: cidr, cidr, interface: iface || '' } });
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
          <select class="select" name="role"><option value="viewer">Solo lectura</option><option value="admin">Administrador</option></select></div>
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
