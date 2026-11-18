'use strict';

/* ============================================================
 * NetDiag SPA — F40: saved views, share links and Ctrl+K palette.
 * Depends on the core helpers declared in app.js (state, api, esc,
 * icon, tr, openModal, toast, navTo and route).
 * ============================================================ */

const SAVED_VIEW_KINDS = ['hosts', 'alerts'];
const SAVED_VIEW_STATUS = {
  hosts: ['', 'all', 'up', 'down', 'unknown', 'archived'],
  alerts: ['', 'all', 'active', 'acked', 'resolved'],
};
const SAVED_VIEW_SEVERITIES = ['', 'info', 'warning', 'critical'];
const SAVED_VIEW_WINDOWS = ['', '24h', '7d', '30d'];
const SAVED_VIEW_SORTS = {
  hosts: ['ip', 'hostname', 'vendor', 'device_type', 'status', 'latency_ms', 'first_seen', 'last_seen'],
  alerts: ['created_at', 'severity', 'status', 'title'],
};
const SAVED_VIEW_COLUMNS = {
  hosts: ['hostname', 'ip', 'mac', 'vendor', 'model', 'device_type', 'status', 'latency_ms', 'open_port_count', 'first_seen', 'last_seen', 'tags'],
  alerts: ['severity', 'status', 'title', 'message', 'host_ip', 'rule_name', 'created_at', 'acked_at', 'resolved_at'],
};
const SAVED_VIEW_LABELS = {
  all: 'Todas', up: 'En línea', down: 'Caído', unknown: 'Desconocido', archived: 'Archivado',
  active: 'Activa', acked: 'Reconocida', resolved: 'Resuelta',
  info: 'Info', warning: 'Aviso', critical: 'Crítico',
  '24h': '24 horas', '7d': '7 días', '30d': '30 días',
  ip: 'IP', hostname: 'Nombre', vendor: 'Fabricante', device_type: 'Tipo', status: 'Estado',
  latency_ms: 'Latencia', first_seen: 'Primera vista', last_seen: 'Última vista',
  created_at: 'Creación', severity: 'Severidad', title: 'Título', model: 'Modelo', mac: 'MAC',
  open_port_count: 'Puertos abiertos', tags: 'Etiquetas', message: 'Mensaje', host_ip: 'IP de host',
  rule_name: 'Regla', acked_at: 'Reconocimiento', resolved_at: 'Resolución',
};

let paletteState = { open: false, query: '' };
let paletteResults = [];
let paletteIndex = 0;

const viewText = (s) => tr(String(s === null || s === undefined ? '' : s));
const viewLabel = (key) => SAVED_VIEW_LABELS[key] ? tr(SAVED_VIEW_LABELS[key]) : String(key || '');

function cleanShareValue(value, max = 200) {
  const text = String(value === null || value === undefined ? '' : value)
    .replace(/[\u0000-\u001f\u007f]/g, ' ')
    .trim();
  return Array.from(text).slice(0, max).join('');
}

function inList(value, list, fallback = '') {
  return list.includes(value) ? value : fallback;
}

function showViewColumn(kind, column) {
  const active = state.views.activeColumns && state.views.activeColumns[kind];
  const cols = Array.isArray(active) && active.length ? active : SAVED_VIEW_COLUMNS[kind];
  return cols.includes(column);
}

function viewCatalogFor(kind) {
  const cat = state.views.catalog || {};
  const k = SAVED_VIEW_KINDS.includes(kind) ? kind : 'hosts';
  return {
    columns: (cat.columns && Array.isArray(cat.columns[k]) && cat.columns[k].length) ? cat.columns[k] : SAVED_VIEW_COLUMNS[k],
    sorts: (cat.sorts && Array.isArray(cat.sorts[k]) && cat.sorts[k].length) ? cat.sorts[k] : SAVED_VIEW_SORTS[k],
    statuses: (cat.statuses && Array.isArray(cat.statuses[k]) && cat.statuses[k].length) ? cat.statuses[k] : SAVED_VIEW_STATUS[k],
    severities: Array.isArray(cat.severities) && cat.severities.length ? cat.severities : SAVED_VIEW_SEVERITIES,
    windows: Array.isArray(cat.windows) && cat.windows.length ? cat.windows : SAVED_VIEW_WINDOWS,
    maxViews: Number(cat.max_views) || 100,
  };
}

/* ---------------- shared URLs ---------------- */

// Applies a hash query such as #/hosts?q=...&status=down. Only allowlisted,
// length-capped values reach the UI state; HTML is escaped later by esc().
function applySharedFilters(view, q) {
  if (!q) return;
  const siteRaw = cleanShareValue(q.get('site_id'), 20);
  if (/^\d+$/.test(siteRaw)) {
    const siteId = Number(siteRaw);
    if (state.sites.some((s) => Number(s.id) === siteId)) {
      state.siteId = siteId;
      try { localStorage.setItem('netdiag-site', String(siteId)); } catch {}
      updateSiteSelect();
    }
  }
  const desc = q.get('desc') === '1' || /^true$/i.test(q.get('desc') || '');
  const window = inList(cleanShareValue(q.get('window'), 20), SAVED_VIEW_WINDOWS);
  state.views.activeWindow = window;
  if (SAVED_VIEW_KINDS.includes(view)) {
    const rawCols = cleanShareValue(q.get('columns'), 300).split(',').map((c) => cleanShareValue(c, 40));
    const cols = rawCols.filter((c) => SAVED_VIEW_COLUMNS[view].includes(c)).slice(0, 16);
    state.views.activeColumns[view] = cols.length ? cols : null;
  }
  if (view === 'hosts') {
    state.hosts.filters.search = cleanShareValue(q.get('q'), 200);
    const status = inList(cleanShareValue(q.get('status'), 30), SAVED_VIEW_STATUS.hosts);
    state.hosts.filters.status = status === 'all' ? '' : status;
    const type = cleanShareValue(q.get('device_type'), 40);
    state.hosts.filters.device_type = (typeof TYPE_LABELS !== 'undefined' && TYPE_LABELS[type]) ? type : '';
    state.hosts.sort = inList(cleanShareValue(q.get('sort'), 40), SAVED_VIEW_SORTS.hosts, 'ip');
    state.hosts.asc = !desc;
    state.hosts.page = 1;
  } else if (view === 'alerts') {
    state.alerts.search = cleanShareValue(q.get('q'), 200);
    const status = inList(cleanShareValue(q.get('status'), 30), SAVED_VIEW_STATUS.alerts, 'active');
    state.alerts.status = status === 'all' ? '' : status;
    state.alerts.severity = inList(cleanShareValue(q.get('severity'), 30), SAVED_VIEW_SEVERITIES);
    state.alerts.page = 1;
    state.alertsTab = 'alertas';
  } else if (view === 'views') {
    state.views.kind = inList(cleanShareValue(q.get('kind'), 20), SAVED_VIEW_KINDS);
  }
}

/* ---------------- filters -> view draft ---------------- */

function currentViewDraft(kind) {
  const k = SAVED_VIEW_KINDS.includes(kind) ? kind : 'hosts';
  const base = {
    id: 0, name: '', kind: k, site_id: state.siteId || 0,
    query: '', severity: '', status: '', window: '',
    columns: SAVED_VIEW_COLUMNS[k].slice(), sort: SAVED_VIEW_SORTS[k][0], desc: true,
  };
  if (k === 'hosts') {
    base.query = state.hosts.filters.search || '';
    base.status = state.hosts.filters.status || '';
    base.sort = state.hosts.sort || 'ip';
    base.desc = !state.hosts.asc;
  } else {
    base.query = state.alerts.search || '';
    base.status = state.alerts.status || 'active';
    base.severity = state.alerts.severity || '';
    base.sort = 'created_at';
  }
  return base;
}

function viewSummary(v) {
  const parts = [];
  if (v.query) parts.push(`«${v.query}»`);
  if (v.status && v.status !== 'all') parts.push(viewLabel(v.status));
  if (v.severity) parts.push(viewLabel(v.severity));
  if (v.window) parts.push(viewLabel(v.window));
  parts.push(v.desc ? tr('descendente') : tr('ascendente'));
  return parts.join(' · ');
}

function savedViewBar(kind) {
  const items = state.views.items.filter((v) => v.kind === kind);
  const options = items.map((v) => `<option value="${v.id}">${esc(v.name)}</option>`).join('');
  return `<div class="saved-view-bar">
    <select class="select sm" data-view-apply ${items.length ? '' : 'disabled'} aria-label="${esc(viewText('Aplicar vista guardada'))}">
      <option value="">${esc(viewText('Vistas guardadas…'))}</option>${options}
    </select>
    <button class="btn sm" data-action="view-save" data-kind="${kind}" title="${esc(viewText('Guardar filtros actuales'))}">${icon('plus')} ${esc(viewText('Guardar'))}</button>
    <button class="btn sm ghost icon" data-action="view-manage" data-kind="${kind}" title="${esc(viewText('Gestionar vistas guardadas'))}">${icon('star')}</button>
  </div>`;
}

/* ---------------- load & list ---------------- */

async function loadSavedViews(force = false) {
  if (state.views.loaded && !force) return state.views.items;
  try {
    const [list, catalog] = await Promise.all([api('/views'), api('/views/catalog')]);
    state.views.items = list.items || [];
    state.views.catalog = catalog || null;
    state.views.loaded = true;
  } catch (e) {
    if (e && e.status === 401) return state.views.items;
    if (!force) state.views.items = [];
  }
  return state.views.items;
}

function filteredViews() {
  const kind = state.views.kind || '';
  const needle = (state.views.search || '').trim().toLocaleLowerCase();
  return state.views.items.filter((v) => (!kind || v.kind === kind)
    && (!needle || String(v.name).toLocaleLowerCase().includes(needle)
      || String(v.query || '').toLocaleLowerCase().includes(needle)));
}

function viewsRows(items) {
  if (!items.length) {
    return `<tr><td colspan="5"><div class="empty">${icon('star')}${esc(viewText('No hay vistas guardadas con estos filtros'))}</div></td></tr>`;
  }
  return items.map((v) => {
    const kindLabel = v.kind === 'alerts' ? viewText('Alertas') : viewText('Dispositivos');
    const siteLabel = Number(v.site_id) > 0
      ? esc((state.sites.find((s) => Number(s.id) === Number(v.site_id)) || {}).name || `#${v.site_id}`)
      : viewText('Todas las sedes');
    return `<tr data-view-id="${v.id}">
      <td><div class="name-cell"><b>${esc(v.name)}</b><span>${esc(viewSummary(v))}</span></div></td>
      <td><span class="badge ${v.kind === 'alerts' ? 'violet' : 'info'}">${esc(kindLabel)}</span></td>
      <td class="tiny muted">${siteLabel}</td>
      <td class="tiny">${v.builtin ? `<span class="tag">${esc(viewText('Integrada'))}</span>` : `<span class="tag">${esc(viewText('Personal'))}</span>`}</td>
      <td class="right nowrap">
        <button class="btn sm primary" data-action="view-apply" data-id="${v.id}">${esc(viewText('Aplicar'))}</button>
        <button class="btn sm" data-action="view-edit" data-id="${v.id}">${icon('edit')} ${esc(viewText('Editar'))}</button>
        <button class="btn sm" data-action="view-share" data-id="${v.id}">${icon('copy')} ${esc(viewText('Compartir'))}</button>
        <button class="btn sm danger" data-action="view-delete" data-id="${v.id}" aria-label="${esc(viewText('Eliminar'))}">${icon('trash')}</button>
      </td>
    </tr>`;
  }).join('');
}

async function renderSavedViews() {
  setPageTitle('views');
  const content = $('#content');
  if (!content) return;
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  await loadSavedViews(true);
  const cat = viewCatalogFor('hosts');
  const kind = state.views.kind || '';
  const tabs = [[ '', viewText('Todas') ], [ 'hosts', viewText('Dispositivos') ], [ 'alerts', viewText('Alertas') ]];
  content.innerHTML = `
    <div class="spread mb">
      <div>
        <h2 style="margin:0 0 4px">${esc(viewText('Vistas guardadas'))}</h2>
        <div class="muted small">${esc(viewText('Guarda tus filtros, reutilízalos y compártelos con un enlace seguro.'))}</div>
      </div>
      <div class="row">
        <button class="btn" data-action="view-palette">${icon('search')} ${esc(viewText('Buscar'))} <span class="tag">Ctrl+K</span></button>
        <button class="btn primary" data-action="view-new">${icon('plus')} ${esc(viewText('Nueva vista'))}</button>
      </div>
    </div>
    <div class="card mb">
      <div class="filters">
        ${tabs.map(([k, label]) => `<button class="btn sm ${kind === k ? 'primary' : ''}" data-action="view-kind-filter" data-kind="${k}">${esc(label)}</button>`).join('')}
        <input class="input" data-view-search placeholder="${esc(viewText('Buscar por nombre o consulta…'))}" value="${esc(state.views.search || '')}" maxlength="80">
        <div class="spacer right"></div>
        <span class="muted tiny">${state.views.items.length} / ${cat.maxViews}</span>
      </div>
    </div>
    <div class="card">
      <div class="table-wrap"><table>
        <thead><tr><th>${esc(viewText('Vista'))}</th><th>${esc(viewText('Tipo'))}</th><th>${esc(viewText('Sede'))}</th><th>${esc(viewText('Origen'))}</th><th></th></tr></thead>
        <tbody id="views-table-body">${viewsRows(filteredViews())}</tbody>
      </table></div>
    </div>
    <div class="notice mt">${icon('info')}<div>${esc(viewText('Los enlaces compartidos incluyen los filtros, nunca el identificador de tu vista ni los datos de otros usuarios.'))}</div></div>`;
}

/* ---------------- editor ---------------- */

function statusOptions(kind, current) {
  const list = viewCatalogFor(kind).statuses;
  return list.map((s) => `<option value="${esc(s)}" ${String(current || '') === s ? 'selected' : ''}>${esc(s ? viewLabel(s) : viewText('Todos los estados'))}</option>`).join('');
}

function sortOptions(kind, current) {
  const list = viewCatalogFor(kind).sorts;
  return list.map((s) => `<option value="${esc(s)}" ${String(current || '') === s ? 'selected' : ''}>${esc(viewLabel(s))}</option>`).join('');
}

function columnsHTML(kind, current) {
  const cols = viewCatalogFor(kind).columns;
  const selected = Array.isArray(current) && current.length ? current : cols;
  return cols.map((c) => `<label class="check tiny view-column"><input type="checkbox" name="columns" value="${esc(c)}" ${selected.includes(c) ? 'checked' : ''}> ${esc(viewLabel(c))}</label>`).join('');
}

function viewFormHTML(v) {
  const k = SAVED_VIEW_KINDS.includes(v.kind) ? v.kind : 'hosts';
  const sites = [{ id: 0, name: viewText('Todas las sedes') }].concat(state.sites || []);
  return `<form data-form="view" data-id="${v.id || ''}" id="saved-view-form">
    <div class="grid cols-2">
      <div class="field"><label for="view-name">${esc(viewText('Nombre'))}</label>
        <input class="input" id="view-name" name="name" maxlength="80" required value="${esc(v.name || '')}" placeholder="${esc(viewText('Ej.: Críticas de LAN'))}"></div>
      <div class="field"><label for="view-kind">${esc(viewText('Tipo'))}</label>
        <select class="select" id="view-kind" name="kind" data-view-form-kind>
          ${SAVED_VIEW_KINDS.map((kind) => `<option value="${kind}" ${k === kind ? 'selected' : ''}>${esc(kind === 'alerts' ? viewText('Alertas') : viewText('Dispositivos'))}</option>`).join('')}
        </select></div>
      <div class="field"><label for="view-site">${esc(viewText('Sede'))}</label>
        <select class="select" id="view-site" name="site_id">
          ${sites.map((s) => `<option value="${s.id}" ${Number(s.id) === Number(v.site_id || 0) ? 'selected' : ''}>${esc(s.name)}</option>`).join('')}
        </select></div>
      <div class="field" data-view-status-field><label for="view-status">${esc(viewText('Estado'))}</label>
        <select class="select" id="view-status" name="status" data-view-form-status>${statusOptions(k, v.status)}</select></div>
      <div class="field" data-view-severity-field ${k === 'alerts' ? '' : 'hidden'}><label for="view-severity">${esc(viewText('Severidad'))}</label>
        <select class="select" id="view-severity" name="severity" data-view-form-severity>
          ${viewCatalogFor('alerts').severities.map((s) => `<option value="${esc(s)}" ${(v.severity || '') === s ? 'selected' : ''}>${esc(s ? viewLabel(s) : viewText('Toda severidad'))}</option>`).join('')}
        </select></div>
      <div class="field"><label for="view-window">${esc(viewText('Ventana temporal'))}</label>
        <select class="select" id="view-window" name="window">
          ${viewCatalogFor(k).windows.map((w) => `<option value="${esc(w)}" ${(v.window || '') === w ? 'selected' : ''}>${esc(w ? viewLabel(w) : viewText('Sin límite'))}</option>`).join('')}
        </select></div>
      <div class="field"><label for="view-sort">${esc(viewText('Ordenar por'))}</label>
        <select class="select" id="view-sort" name="sort" data-view-form-sort>${sortOptions(k, v.sort)}</select></div>
      <div class="field"><label>${esc(viewText('Dirección'))}</label>
        <label class="check"><input type="checkbox" name="desc" ${v.desc ? 'checked' : ''}> ${esc(viewText('Descendente'))}</label></div>
    </div>
    <div class="field"><label for="view-query">${esc(viewText('Consulta (texto libre)'))}</label>
      <input class="input" id="view-query" name="query" maxlength="200" value="${esc(v.query || '')}" placeholder="${esc(viewText('IP, nombre, MAC…'))}"></div>
    <div class="field"><label>${esc(viewText('Columnas'))}</label>
      <div class="view-columns" data-view-form-columns>${columnsHTML(k, v.columns)}</div></div>
    <p class="muted tiny">${esc(viewText('Máximo 80 caracteres para el nombre, 200 para la consulta y 16 columnas.'))}</p>
  </form>`;
}

function refreshViewForm(form) {
  if (!form) return;
  const kind = form.querySelector('[data-view-form-kind]').value;
  const k = SAVED_VIEW_KINDS.includes(kind) ? kind : 'hosts';
  const statusField = form.querySelector('[data-view-form-status]');
  const severityField = form.querySelector('[data-view-severity-field]');
  const sortField = form.querySelector('[data-view-form-sort]');
  const columnsField = form.querySelector('[data-view-form-columns]');
  const windowField = form.querySelector('[name="window"]');
  statusField.innerHTML = statusOptions(k, statusField.value);
  sortField.innerHTML = sortOptions(k, sortField.value);
  columnsField.innerHTML = columnsHTML(k, Array.from(form.querySelectorAll('[name="columns"]:checked')).map((el) => el.value));
  if (severityField) severityField.hidden = k !== 'alerts';
  if (windowField) {
    const current = windowField.value;
    windowField.innerHTML = viewCatalogFor(k).windows.map((w) =>
      `<option value="${esc(w)}" ${current === w ? 'selected' : ''}>${esc(w ? viewLabel(w) : viewText('Sin límite'))}</option>`).join('');
  }
}

function openViewModal(view, kind) {
  const draft = view ? Object.assign({}, view) : currentViewDraft(kind || state.views.kind || (state.view === 'alerts' ? 'alerts' : 'hosts'));
  if (!view && !draft.name) draft.name = draft.kind === 'alerts' ? viewText('Mis alertas') : viewText('Mis dispositivos');
  const footer = `<button class="btn" data-action="modal-close">${esc(viewText('Cancelar'))}</button>
    <button class="btn primary" type="submit" form="saved-view-form">${esc(viewText('Guardar'))}</button>`;
  openModal(view ? viewText('Editar vista') : viewText('Nueva vista'), viewFormHTML(draft), { wide: true, footer });
}

function viewBodyFromForm(form) {
  const fd = new FormData(form);
  const kind = String(fd.get('kind') || 'hosts');
  return {
    name: String(fd.get('name') || '').trim(),
    kind,
    site_id: Number(fd.get('site_id') || 0) || 0,
    query: String(fd.get('query') || '').trim(),
    severity: kind === 'alerts' ? String(fd.get('severity') || '') : '',
    status: String(fd.get('status') || ''),
    window: String(fd.get('window') || ''),
    columns: fd.getAll('columns').slice(0, 16),
    sort: String(fd.get('sort') || ''),
    desc: fd.has('desc'),
  };
}

async function submitViewForm(form) {
  const id = Number(form.dataset.id || 0) || 0;
  const body = viewBodyFromForm(form);
  if (!body.name) { toast(viewText('El nombre es obligatorio'), { type: 'err' }); return; }
  if (id) await api('/views/' + id, { method: 'PUT', body });
  else await api('/views', { method: 'POST', body });
  closeModal();
  toast(id ? viewText('Vista actualizada') : viewText('Vista guardada'), { type: 'ok' });
  await loadSavedViews(true);
  if (state.view === 'views') renderSavedViews();
  else route(true);
}

/* ---------------- actions ---------------- */

async function applySavedView(id) {
  const viewId = Number(id) || 0;
  if (!viewId) return;
  const share = await api('/views/' + viewId + '/share');
  closeModal();
  closeCommandPalette();
  if (location.hash === share.path) route();
  else location.hash = share.path;
}

async function shareSavedView(id) {
  const share = await api('/views/' + id + '/share');
  const url = location.origin + location.pathname + share.path;
  openModal(viewText('Enlace compartido'), `<p class="muted small">${esc(viewText('Cualquier usuario autenticado puede abrir este enlace y ver los mismos filtros.'))}</p>
    <input class="input" id="view-share-url" readonly value="${esc(url)}">
    <p class="muted tiny">${esc(viewText('El enlace no contiene tu identificador de vista ni datos privados.'))}</p>`,
  { footer: `<button class="btn" data-action="modal-close">${esc(viewText('Cerrar'))}</button>
    <button class="btn primary" data-action="copy-text" data-text="${esc(url)}">${icon('copy')} ${esc(viewText('Copiar enlace'))}</button>` });
}

async function deleteSavedView(id) {
  if (!window.confirm(viewText('¿Eliminar esta vista?'))) return;
  await api('/views/' + id, { method: 'DELETE' });
  await loadSavedViews(true);
  toast(viewText('Vista eliminada'), { type: 'ok' });
  if (state.view === 'views') renderSavedViews();
  else route(true);
}

/* ---------------- command palette (Ctrl+K) ---------------- */

function paletteRoot() {
  let root = document.getElementById('command-palette-root');
  if (!root) {
    root = document.createElement('div');
    root.id = 'command-palette-root';
    document.body.appendChild(root);
  }
  return root;
}

function paletteEntries() {
  const entries = [];
  if (typeof NAV !== 'undefined') {
    NAV.filter((n) => n.view && (!n.adminOnly || isAdmin())).forEach((n) => {
      entries.push({ type: 'nav', view: n.view, title: viewText(n.label), sub: viewText('Ir a la página') });
    });
  }
  state.views.items.forEach((v) => {
    entries.push({ type: 'view', id: v.id, title: v.name,
      sub: (v.kind === 'alerts' ? viewText('Alertas') : viewText('Dispositivos')) + ' · ' + viewSummary(v) });
  });
  [
    { key: 'scan', title: 'Escanear red', icon: 'radar' },
    { key: 'new-view', title: 'Crear vista guardada', icon: 'plus' },
    { key: 'theme', title: 'Cambiar tema', icon: 'sun' },
    { key: 'lang', title: 'Cambiar idioma', icon: 'globe' },
  ].forEach((a) => entries.push({ type: 'action', key: a.key, icon: a.icon, title: viewText(a.title), sub: viewText('Acción rápida') }));
  return entries;
}

function paletteListHTML() {
  const needle = paletteState.query.trim().toLocaleLowerCase();
  paletteResults = paletteEntries().filter((it) => !needle
    || it.title.toLocaleLowerCase().includes(needle)
    || it.sub.toLocaleLowerCase().includes(needle)).slice(0, 12);
  paletteIndex = 0;
  if (!paletteResults.length) {
    return `<div class="palette-empty">${esc(viewText('Sin resultados'))}</div>`;
  }
  return paletteResults.map((it, idx) => `<button class="palette-item ${idx === 0 ? 'active' : ''}" data-action="palette-run" data-index="${idx}">
    ${icon(it.icon || (it.type === 'view' ? 'star' : 'book'))}
    <span><b>${esc(it.title)}</b><small>${esc(it.sub)}</small></span>
    <span class="tag">${esc(it.type === 'view' ? viewText('Vista') : it.type === 'nav' ? viewText('Ir') : viewText('Acción'))}</span>
  </button>`).join('');
}

function renderCommandPalette() {
  const root = paletteRoot();
  root.innerHTML = `<div class="palette-backdrop" data-palette-backdrop>
    <div class="palette" role="dialog" aria-modal="true" aria-label="${esc(viewText('Buscar y ejecutar'))}">
      <div class="palette-search">${icon('search')}
        <input id="palette-input" class="input" autocomplete="off" maxlength="80"
          placeholder="${esc(viewText('Buscar páginas, vistas o acciones…'))}" value="${esc(paletteState.query)}">
        <button class="btn ghost icon sm" data-action="palette-close" aria-label="${esc(viewText('Cerrar'))}">${icon('x')}</button>
      </div>
      <div class="palette-list" id="palette-list">${paletteListHTML()}</div>
      <div class="palette-foot"><span>↑ ↓ ${esc(viewText('navegar'))}</span><span>↵ ${esc(viewText('abrir'))}</span><span>Esc ${esc(viewText('cerrar'))}</span></div>
    </div>
  </div>`;
  const backdrop = root.querySelector('[data-palette-backdrop]');
  if (backdrop) backdrop.addEventListener('click', (e) => { if (e.target === backdrop) closeCommandPalette(); });
  const input = document.getElementById('palette-input');
  if (input) { input.focus(); input.setSelectionRange(input.value.length, input.value.length); }
}

function closeCommandPalette() {
  paletteState.open = false;
  const root = document.getElementById('command-palette-root');
  if (root) root.innerHTML = '';
}

function openCommandPalette() {
  paletteState.open = true;
  renderCommandPalette();
}

function movePalette(delta) {
  if (!paletteResults.length) return;
  paletteIndex = (paletteIndex + delta + paletteResults.length) % paletteResults.length;
  $$('.palette-item').forEach((el, idx) => el.classList.toggle('active', idx === paletteIndex));
  const active = document.querySelector('.palette-item.active');
  if (active && active.scrollIntoView) active.scrollIntoView({ block: 'nearest' });
}

async function runPalette(index) {
  const item = paletteResults[Number(index)];
  if (!item) return;
  closeCommandPalette();
  if (item.type === 'nav') { navTo('#/' + item.view); return; }
  if (item.type === 'view') { await applySavedView(item.id); return; }
  switch (item.key) {
    case 'scan': openScanModal(); break;
    case 'new-view': openViewModal(null, state.view === 'alerts' ? 'alerts' : 'hosts'); break;
    case 'theme': toggleTheme(); break;
    case 'lang': if (window.NetDiagI18n) window.NetDiagI18n.toggle(); break;
  }
}

/* ---------------- listeners ---------------- */

document.addEventListener('click', async (e) => {
  const target = e.target.closest('[data-action]');
  if (!target) return;
  const action = target.dataset.action;
  try {
    switch (action) {
      case 'view-palette': e.preventDefault(); openCommandPalette(); break;
      case 'palette-close': closeCommandPalette(); break;
      case 'palette-run': await runPalette(target.dataset.index); break;
      case 'view-save': openViewModal(null, target.dataset.kind); break;
      case 'view-new': openViewModal(null, state.views.kind || state.view); break;
      case 'view-manage': state.views.kind = target.dataset.kind || ''; navTo('#/views'); break;
      case 'view-kind-filter': state.views.kind = target.dataset.kind || ''; renderSavedViews(); break;
      case 'view-apply': await applySavedView(target.dataset.id); break;
      case 'view-edit': {
        const v = await api('/views/' + target.dataset.id);
        openViewModal(v, v.kind);
        break;
      }
      case 'view-share': await shareSavedView(target.dataset.id); break;
      case 'view-delete': await deleteSavedView(target.dataset.id); break;
      default: return;
    }
  } catch (err) {
    if (err && err.status === 401) return;
    toast(err.message || String(err), { type: 'err', title: viewText('Error') });
  }
});

document.addEventListener('change', (e) => {
  const el = e.target;
  if (el.matches && el.matches('[data-view-apply]')) {
    const id = el.value;
    el.value = '';
    if (id) applySavedView(id).catch((err) => toast(err.message || String(err), { type: 'err' }));
    return;
  }
  if (el.matches && el.matches('[data-view-form-kind]')) refreshViewForm(el.form);
  if (el.matches && el.matches('[name="columns"]') && el.form) {
    const checked = Array.from(el.form.querySelectorAll('[name="columns"]:checked'));
    if (checked.length > 16) el.checked = false;
  }
});

document.addEventListener('input', (e) => {
  if (e.target && e.target.id === 'palette-input') {
    paletteState.query = e.target.value;
    const list = document.getElementById('palette-list');
    if (list) list.innerHTML = paletteListHTML();
    return;
  }
  if (e.target && e.target.matches && e.target.matches('[data-view-search]')) {
    state.views.search = e.target.value;
    const body = document.getElementById('views-table-body');
    if (body) body.innerHTML = viewsRows(filteredViews());
  }
});

document.addEventListener('keydown', (e) => {
  const key = (e.key || '').toLowerCase();
  if ((e.ctrlKey || e.metaKey) && key === 'k') {
    e.preventDefault();
    if (paletteState.open) closeCommandPalette();
    else openCommandPalette();
    return;
  }
  if (!paletteState.open) return;
  if (e.key === 'Escape') { e.preventDefault(); closeCommandPalette(); }
  else if (e.key === 'ArrowDown') { e.preventDefault(); movePalette(1); }
  else if (e.key === 'ArrowUp') { e.preventDefault(); movePalette(-1); }
  else if (e.key === 'Enter') { e.preventDefault(); runPalette(paletteIndex); }
});

document.addEventListener('submit', async (e) => {
  const form = e.target.closest('form[data-form="view"]');
  if (!form) return;
  e.preventDefault();
  e.stopPropagation();
  try { await submitViewForm(form); }
  catch (err) {
    if (err && err.status === 401) return;
    toast(err.message || String(err), { type: 'err', title: viewText('Error') });
  }
});
