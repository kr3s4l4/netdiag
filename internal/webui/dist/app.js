'use strict';

/* ============================================================
 * NetDiag SPA — core, authentication, shell, dashboard, hosts.
 * No external dependencies; everything is served from the binary.
 * ============================================================ */

const state = {
  auth: null,
  me: null,
  csrf: '',
  view: 'dashboard',
  networks: [],
  sites: [],
  siteId: 0,
  stats: null,
  hosts: {
    page: 1, limit: 50, total: 0, items: [],
    sort: 'ip', asc: true,
    filters: { search: '', status: '', device_type: '', network_id: '', favorite: false },
  },
  host: null,
  hostHistoryRange: '24h',
  hostTab: 'resumen',
  alerts: { page: 1, limit: 50, status: 'active', severity: '', search: '' },
  rules: [],
  channels: [],
  channelMeta: { types: [], secret_fields: {} },
  integrations: [],
  integrationsLoaded: false,
  integrationMeta: { types: [], secret_fields: {} },
  scanActive: {},
  topo: null,
  topoView: { x: 0, y: 0, k: 1 },
  settingsTab: 'general',
  ws: null,
  wsRetry: 0,
  wsTimer: null,
  refreshTimer: null,
  clockTimer: null,
};

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

// Spanish is the source language; i18n.js translates rendered text to English.
const tr = (s, vars) => (window.NetDiagI18n ? window.NetDiagI18n.t(s, vars) : s);
const dateLocale = () => (window.NetDiagI18n ? window.NetDiagI18n.dateLocale() : 'es-ES');
const roleOf = () => (state.me && state.me.role) || 'admin';
const isAdmin = () => ['admin', 'owner'].includes(roleOf());
const canWrite = () => ['admin', 'owner', 'operator'].includes(roleOf());
const roleName = (r) => ({ admin: 'Administrador', owner: 'Administrador', operator: 'Operador', viewer: 'Solo lectura' }[r] || r || '');

const esc = (v) => String(v === null || v === undefined ? '' : v)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');

const TYPE_LABELS = {
  router: 'Router', switch: 'Switch', access_point: 'Punto de acceso', firewall: 'Firewall',
  server: 'Servidor', workstation: 'Estación', laptop: 'Portátil', phone: 'Teléfono',
  tablet: 'Tablet', printer: 'Impresora', camera: 'Cámara', iot: 'IoT', nas: 'NAS',
  virtual: 'Virtual', media: 'Multimedia', game_console: 'Consola', unknown: 'Desconocido',
};
const STATUS_LABELS = { up: 'En línea', down: 'Caído', unknown: 'Desconocido', archived: 'Archivado' };
const SEV_LABELS = { info: 'Info', warning: 'Aviso', critical: 'Crítico' };
const SCAN_LABELS = {
  queued: 'En cola', running: 'En curso', done: 'Completado', canceled: 'Cancelado',
  error: 'Error', interrupted: 'Interrumpido',
};
const RULE_LABELS = {
  host_down: 'Host caído', host_new: 'Host nuevo', latency_high: 'Latencia alta',
  port_opened: 'Puerto abierto', port_closed: 'Puerto cerrado', service_changed: 'Servicio cambiado',
  device_changed: 'Dispositivo cambiado', subnet_usage: 'Uso de subred', scan_failed: 'Escaneo fallido',
};
const CHANNEL_LABELS = {
  webhook: 'Webhook', slack: 'Slack', discord: 'Discord', telegram: 'Telegram',
  pagerduty: 'PagerDuty', smtp: 'Correo SMTP', syslog: 'Syslog',
};

/* ---------------- icons ---------------- */
const ICONS = {
  dashboard: '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
  hosts: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>',
  topology: '<circle cx="12" cy="5" r="2.6"/><circle cx="5" cy="18" r="2.6"/><circle cx="19" cy="18" r="2.6"/><path d="M10.6 7.2 6.4 15.6M13.4 7.2l4.2 8.4M7.6 18h8.8"/>',
  diagnostics: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.2-3.2M8.5 11h5M11 8.5v5"/>',
  alerts: '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M10.3 21a2 2 0 0 0 3.4 0"/>',
  scans: '<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9 17 7M7 17l-2.1 2.1"/>',
  reports: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6M8 13h8M8 17h5"/>',
  book: '<path d="M4 4a2 2 0 0 1 2-2h12v20H6a2 2 0 0 1-2-2z"/><path d="M8 2v20M12 7h4M12 11h4"/>',
  settings: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.9 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.9V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.2-3.2"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  refresh: '<path d="M21 12a9 9 0 1 1-2.6-6.4M21 3v6h-6"/>',
  download: '<path d="M12 3v12m0 0 4-4m-4 4-4-4M4 21h16"/>',
  trash: '<path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v5M14 11v5"/>',
  star: '<path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2L12 17.3 6.4 20.2l1.1-6.2L3 9.6l6.2-.9z"/>',
  radar: '<path d="M12 3a9 9 0 1 0 9 9"/><path d="M12 8a4 4 0 1 0 4 4"/><path d="M12 12 21 3"/>',
  bolt: '<path d="M13 2 4 14h6l-1 8 9-12h-6z"/>',
  check: '<path d="m4 12 5 5L20 6"/>',
  x: '<path d="M6 6l12 12M18 6 6 18"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  wifi: '<path d="M2 8.8a16 16 0 0 1 20 0M5 12.5a11 11 0 0 1 14 0M8.5 16a6 6 0 0 1 7 0"/><circle cx="12" cy="20" r="1"/>',
  server: '<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01"/>',
  router: '<rect x="2" y="13" width="20" height="7" rx="2"/><path d="M6 16h.01M10 16h.01M12 13V8M12 8 8 4M12 8l4-4"/>',
  switch: '<rect x="2" y="9" width="20" height="6" rx="2"/><path d="M6 12h.01M10 12h.01M14 12h.01M18 12h.01"/>',
  firewall: '<path d="M12 2 4 5v6c0 5 3.4 9.4 8 11 4.6-1.6 8-6 8-11V5z"/><path d="M9 12h6"/>',
  printer: '<path d="M6 9V3h12v6M6 18H4v-7h16v7h-2"/><rect x="7" y="14" width="10" height="7"/>',
  camera: '<path d="M3 7h4l2-2h6l2 2h4v12H3z"/><circle cx="12" cy="13" r="3.5"/>',
  phone: '<rect x="6" y="2" width="12" height="20" rx="3"/><path d="M10 18h4"/>',
  laptop: '<rect x="4" y="4" width="16" height="12" rx="2"/><path d="M2 20h20"/>',
  unknown: '<circle cx="12" cy="12" r="9"/><path d="M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.7v.5"/><path d="M12 17h.01"/>',
  network: '<rect x="9" y="2" width="6" height="5" rx="1"/><rect x="2" y="17" width="6" height="5" rx="1"/><rect x="16" y="17" width="6" height="5" rx="1"/><path d="M12 7v4M5 17v-3h14v3"/>',
  shield: '<path d="M12 2 4 5v6c0 5 3.4 9.4 8 11 4.6-1.6 8-6 8-11V5z"/>',
  menu: '<path d="M4 7h16M4 12h16M4 17h16"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4"/>',
  moon: '<path d="M21 13A9 9 0 1 1 11 3a7 7 0 0 0 10 10z"/>',
  contrast: '<circle cx="12" cy="12" r="9"/><path d="M12 3a9 9 0 0 1 0 18z" fill="currentColor" stroke="none"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9"/>',
  play: '<path d="m7 4 13 8-13 8z"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="1"/>',
  test: '<path d="M9 2v6L4 19a2 2 0 0 0 1.8 3h12.4A2 2 0 0 0 20 19L15 8V2M8 2h8M6.5 15h11"/>',
  edit: '<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>',
  eye: '<path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12z"/><circle cx="12" cy="12" r="3"/>',
  copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h8"/>',
  warn: '<path d="M12 3 2 21h20z"/><path d="M12 9v5M12 18h.01"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
  arrowUp: '<path d="M12 19V5M6 11l6-6 6 6"/>',
  arrowDown: '<path d="M12 5v14M6 13l6 6 6-6"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a15 15 0 0 1 0 18 15 15 0 0 1 0-18z"/>',
};

function icon(name, cls = '') {
  return `<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name] || ICONS.unknown}</svg>`;
}

function typeIcon(type) {
  const map = {
    router: 'router', switch: 'switch', access_point: 'wifi', firewall: 'firewall',
    server: 'server', workstation: 'hosts', laptop: 'laptop', phone: 'phone', tablet: 'phone',
    printer: 'printer', camera: 'camera', nas: 'server', virtual: 'server', iot: 'wifi',
    media: 'hosts', game_console: 'hosts',
  };
  return icon(map[type] || 'unknown');
}

/* ---------------- formatting ---------------- */
function fmtDate(v, time = true) {
  if (!v) return '—';
  const d = new Date(v);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '—';
  return d.toLocaleString(dateLocale(), time ? { dateStyle: 'short', timeStyle: 'short' } : { dateStyle: 'medium' });
}
function fmtTime(v) {
  if (!v) return '—';
  const d = new Date(v);
  if (isNaN(d.getTime())) return '—';
  return d.toLocaleTimeString(dateLocale());
}
function ago(v) {
  if (!v) return '—';
  const d = new Date(v);
  if (isNaN(d.getTime())) return '—';
  const s = Math.max(0, (Date.now() - d.getTime()) / 1000);
  if (s < 45) return tr('ahora');
  if (s < 90) return tr('hace 1 min');
  if (s < 3600) return tr('hace {n} min', { n: Math.round(s / 60) });
  if (s < 86400) return tr('hace {n} h', { n: Math.round(s / 3600) });
  return tr('hace {n} d', { n: Math.round(s / 86400) });
}
function fmtDuration(a, b) {
  if (!a || !b) return '—';
  const s = Math.max(0, (new Date(b) - new Date(a)) / 1000);
  if (s < 60) return `${Math.round(s)} s`;
  const m = Math.floor(s / 60), r = Math.round(s % 60);
  if (m < 60) return `${m} m ${r} s`;
  return `${Math.floor(m / 60)} h ${m % 60} m`;
}
function fmtBytes(n) {
  if (!n) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0, v = n;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(i ? 1 : 0)} ${units[i]}`;
}
function fmtNum(n, digits = 0) {
  if (n === null || n === undefined || isNaN(n)) return '—';
  return Number(n).toLocaleString(dateLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits });
}
function typeLabel(t) { return TYPE_LABELS[t] || t || tr('Desconocido'); }
function statusBadge(s) {
  return `<span class="badge ${esc(s || 'unknown')}"><i class="dot"></i>${esc(STATUS_LABELS[s] || s || 'Desconocido')}</span>`;
}
function statusDot(s) {
  return `<span class="sev-dot" style="background:${s === 'up' ? 'var(--green)' : s === 'down' ? 'var(--red)' : 'var(--muted-2)'}"></span>`;
}
function sevBadge(s) { return `<span class="badge ${esc(s)}">${esc(SEV_LABELS[s] || s)}</span>`; }
function scanBadge(s) { return `<span class="badge ${esc(s)}">${esc(SCAN_LABELS[s] || s)}</span>`; }
function boolTag(v, yes = 'Sí', no = 'No') { return v ? yes : no; }

/* ---------------- toasts & modal ---------------- */
function toast(message, opts = {}) {
  const root = $('#toast-root');
  if (!root) return;
  const el = document.createElement('div');
  el.className = `toast ${opts.type || ''}`;
  el.innerHTML = `<div>${opts.title ? `<b>${esc(opts.title)}</b>` : ''}<span>${esc(message)}</span></div>`;
  root.appendChild(el);
  const ttl = opts.ttl || (opts.type === 'err' ? 8000 : 4500);
  setTimeout(() => { el.style.opacity = '0'; el.style.transform = 'translateX(12px)'; el.style.transition = 'all .2s'; }, ttl);
  setTimeout(() => el.remove(), ttl + 220);
}

let modalCleanup = null;
function openModal(title, bodyHTML, opts = {}) {
  closeModal();
  const root = $('#modal-root');
  root.innerHTML = `<div class="modal-backdrop" data-modal-backdrop>
    <div class="modal ${opts.wide ? 'wide' : ''}" role="dialog" aria-modal="true">
      <div class="modal-head"><h3>${esc(title)}</h3>
        <button class="btn ghost icon sm" data-action="modal-close" aria-label="Cerrar">${icon('x')}</button>
      </div>
      <div class="modal-body">${bodyHTML}</div>
      ${opts.footer ? `<div class="modal-foot">${opts.footer}</div>` : ''}
    </div>
  </div>`;
  const backdrop = $('[data-modal-backdrop]', root);
  const onKey = (e) => { if (e.key === 'Escape') closeModal(); };
  const onClick = (e) => { if (e.target === backdrop) closeModal(); };
  document.addEventListener('keydown', onKey);
  backdrop.addEventListener('click', onClick);
  modalCleanup = () => {
    document.removeEventListener('keydown', onKey);
    backdrop.removeEventListener('click', onClick);
  };
  setTimeout(() => {
    const first = $('input,select,textarea,button', root.querySelector('.modal'));
    if (first && opts.autofocus !== false) first.focus();
  }, 40);
  return root.querySelector('.modal');
}
function closeModal() {
  if (modalCleanup) { modalCleanup(); modalCleanup = null; }
  const root = $('#modal-root');
  if (root) root.innerHTML = '';
}

/* ---------------- api ---------------- */
class ApiError extends Error {
  constructor(message, status, code) { super(message); this.status = status; this.code = code; }
}

// The active site scopes every operational query. Endpoints that do not
// understand site_id simply ignore the extra query parameter.
function withSite(path) {
  if (!state.siteId || /^\/auth\//.test(path) || /^\/sites(\/|\?|$)/.test(path)) return path;
  return path + (path.includes('?') ? '&' : '?') + 'site_id=' + encodeURIComponent(state.siteId);
}

function currentSite() {
  return state.sites.find((s) => Number(s.id) === Number(state.siteId)) || null;
}

async function api(path, opts = {}) {
  const headers = { Accept: 'application/json' };
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
  if (state.csrf) headers['X-CSRF-Token'] = state.csrf;
  let res;
  try {
    res = await fetch('/api/v1' + withSite(path), {
      method: opts.method || 'GET',
      headers: Object.assign(headers, opts.headers || {}),
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      credentials: 'same-origin',
      cache: 'no-store',
      signal: opts.signal,
    });
  } catch (e) {
    if (e.name === 'AbortError') throw e;
    throw new ApiError('No se pudo contactar con el servidor', 0, 'network');
  }
  if (res.status === 204) return null;
  const text = await res.text();
  let data = null;
  if (text) { try { data = JSON.parse(text); } catch { data = { error: text }; } }
  if (!res.ok) {
    const msg = (data && (data.error || data.message)) || `Error HTTP ${res.status}`;
    if (res.status === 401 && !opts.noAuthRedirect) {
      state.auth = Object.assign({}, state.auth, { authenticated: false });
      showLogin(state.auth);
    }
    if (res.status === 428 && !opts.noAuthRedirect) {
      showSetup(state.auth);
    }
    throw new ApiError(msg, res.status, data && data.code);
  }
  return data;
}

function debounce(fn, ms = 350) {
  let t;
  return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
}

/* ---------------- auth screens ---------------- */
function renderBootError(err) {
  $('#boot').classList.add('hidden');
  $('#root').innerHTML = `<div class="auth-wrap"><div class="auth-card">
    <div class="auth-logo"><div class="mark">N</div><h1>NetDiag</h1></div>
    <p class="auth-sub">No se pudo inicializar la interfaz.</p>
    <div class="notice err">${esc(err && err.message || err)}</div>
    <p class="auth-foot"><a href="#" data-action="reload">Reintentar</a></p>
  </div></div>`;
}

function renderAuthCard(mode, st) {
  $('#boot').classList.add('hidden');
  const setup = mode === 'setup';
  const title = setup ? 'Configuración inicial' : 'Iniciar sesión';
  const sub = setup
    ? 'Crea la cuenta de administrador. La contraseña debe tener al menos 10 caracteres, con letras y números.'
    : 'Accede al inventario y monitoreo de tu red.';
  $('#root').innerHTML = `<div class="auth-wrap"><div class="auth-card">
    <div class="auth-logo"><div class="mark">N</div><h1>NetDiag</h1></div>
    <p class="auth-sub">${esc(sub)}</p>
    <form data-form="${setup ? 'setup' : 'login'}">
      <div class="field">
        <label for="auth-user">Usuario</label>
        <input class="input" id="auth-user" name="username" autocomplete="username" required maxlength="64" placeholder="admin">
      </div>
      <div class="field">
        <label for="auth-pass">Contraseña</label>
        <input class="input" id="auth-pass" name="password" type="password" autocomplete="${setup ? 'new-password' : 'current-password'}" required minlength="${setup ? 10 : 1}">
      </div>
      ${setup ? `<div class="field"><label for="auth-pass2">Repetir contraseña</label>
        <input class="input" id="auth-pass2" name="password2" type="password" autocomplete="new-password" required minlength="10"></div>` : ''}
      <div id="auth-error" class="notice err hidden"></div>
      <button class="btn primary w-full mt" type="submit">${setup ? 'Crear cuenta y entrar' : 'Entrar'}</button>
    </form>
    <p class="auth-foot">NetDiag ${esc((st && st.version) || '')} · servidor local</p>
  </div></div>`;
  const input = $('#auth-user');
  if (input) input.focus();
}

function showLogin(st) { state.auth = st || state.auth || {}; renderAuthCard('login', st || state.auth); }
function showSetup(st) { state.auth = st || state.auth || {}; renderAuthCard('setup', st || state.auth); }

async function submitAuth(form) {
  const mode = form.dataset.form;
  const fd = new FormData(form);
  const username = String(fd.get('username') || '').trim();
  const password = String(fd.get('password') || '');
  const errorBox = $('#auth-error');
  const btn = form.querySelector('button[type=submit]');
  errorBox.classList.add('hidden');
  if (mode === 'setup' && password !== String(fd.get('password2') || '')) {
    errorBox.textContent = 'Las contraseñas no coinciden';
    errorBox.classList.remove('hidden');
    return;
  }
  btn.disabled = true;
  try {
    const res = await api(mode === 'setup' ? '/auth/setup' : '/auth/login', {
      method: 'POST', body: { username, password }, noAuthRedirect: true,
    });
    state.me = res.user;
    state.csrf = res.csrf || '';
    state.auth = Object.assign({}, state.auth, { authenticated: true, user: res.user, csrf: res.csrf, setup_required: false });
    enterApp();
  } catch (e) {
    errorBox.textContent = e.message;
    errorBox.classList.remove('hidden');
    btn.disabled = false;
  }
}

async function enterApp() {
  $('#boot').classList.add('hidden');
  try { state.siteId = Number(localStorage.getItem('netdiag-site') || 0) || 0; } catch {}
  renderShell();
  await loadSites(true);
  connectWS();
  const current = (location.hash || '').replace(/^#\/?/, '');
  if (!current) location.hash = '#/dashboard';
  else route();
  startClock();
  setTimeout(() => {
    try {
      if (!localStorage.getItem('netdiag-tour-done')) startTour();
    } catch {}
  }, 900);
}

function logout() {
  api('/auth/logout', { method: 'POST', noAuthRedirect: true }).catch(() => {});
  if (state.ws) { try { state.ws.close(); } catch {} state.ws = null; }
  if (state.clockTimer) clearInterval(state.clockTimer);
  state.me = null; state.csrf = ''; state.stats = null;
  showLogin(Object.assign({}, state.auth, { authenticated: false }));
}

/* ---------------- shell ---------------- */
const NAV = [
  { section: 'Operación' },
  { view: 'dashboard', label: 'Panel', icon: 'dashboard' },
  { view: 'hosts', label: 'Dispositivos', icon: 'hosts' },
  { view: 'topology', label: 'Topología', icon: 'topology' },
  { view: 'diagnostics', label: 'Diagnósticos', icon: 'diagnostics' },
  { view: 'dashboards', label: 'Dashboards', icon: 'dashboard' },
  { view: 'wifi', label: 'WiFi', icon: 'wifi' },
  { view: 'vpn', label: 'VPN', icon: 'network' },
  { view: 'alerts', label: 'Alertas', icon: 'alerts', badge: true },
  { section: 'Historial' },
  { view: 'scans', label: 'Escaneos', icon: 'scans' },
  { view: 'reports', label: 'Informes', icon: 'reports' },
  { section: 'Sistema' },
  { view: 'guide', label: 'Guía', icon: 'book' },
  { view: 'settings', label: 'Configuración', icon: 'settings', adminOnly: true },
];

function renderShell() {
  const nav = NAV.filter((item) => !item.adminOnly || isAdmin()).map((item) => {
    if (item.section) return `<div class="nav-section">${esc(tr(item.section))}</div>`;
    return `<a class="nav-item" data-action="nav" data-view="${item.view}" href="#/${item.view}">
      ${icon(item.icon)}<span>${esc(tr(item.label))}</span>
      ${item.badge ? '<span class="count hidden" id="nav-alert-count">0</span>' : ''}
    </a>`;
  }).join('');
  const user = state.me || { username: 'local', role: 'admin' };
  const initials = (user.username || 'N').slice(0, 1).toUpperCase();
  document.documentElement.dataset.canWrite = canWrite() ? '1' : '0';
  document.documentElement.dataset.isAdmin = isAdmin() ? '1' : '0';
  const i18n = window.NetDiagI18n;
  $('#root').innerHTML = `<div class="shell">
    <aside class="sidebar" id="sidebar">
      <div class="brand">
        <div class="mark">N</div>
        <div><div class="name">NetDiag</div><div class="sub">network diagnostics</div></div>
      </div>
      <nav class="nav">${nav}</nav>
      <div class="sidebar-foot">
        <div class="live"><span class="dot" id="live-dot"></span><span id="live-label">Conectando…</span></div>
        <div class="user-mini">
          <div class="avatar">${esc(initials)}</div>
          <div class="who"><b>${esc(user.username)}</b><span>${esc(tr(roleName(user.role || 'admin')))}</span></div>
          <button class="btn ghost icon sm right" data-action="logout" title="Cerrar sesión">${icon('logout')}</button>
        </div>
      </div>
    </aside>
    <div class="main">
      <header class="topbar">
        <button class="btn ghost icon sm" data-action="toggle-sidebar" aria-label="Menú" style="display:none" id="burger">${icon('menu')}</button>
        <h2 id="page-title">Panel</h2>
        <span class="crumb" id="page-crumb"></span>
        <div class="spacer"></div>
        <label class="site-picker" title="${esc(tr('Sede activa'))}">${icon('network')}
          <select class="select sm" id="site-select" aria-label="${esc(tr('Sede activa'))}"></select>
        </label>
        <span class="clock" id="clock">—</span>
        <button class="btn sm ghost lang-btn" data-action="lang" title="Cambiar idioma">${i18n ? i18n.label : 'EN'}</button>
        <button class="btn ghost icon" data-action="theme" title="${esc(t('Cambiar tema'))}" id="theme-btn">${icon(themeIcon(document.documentElement.dataset.theme || 'dark'))}</button>
        <button class="btn primary" data-action="open-scan">${icon('radar')} Escanear</button>
      </header>
      <div class="content" id="content"><div class="loading"><div class="spinner"></div></div></div>
    </div>
  </div>`;
  updateNav();
  refreshAlertBadge();
  const burger = $('#burger');
  if (window.innerWidth <= 820 && burger) burger.style.display = '';
}

async function loadSites(keepSelection = true) {
  try {
    const res = await api('/sites');
    state.sites = res.items || [];
  } catch {
    state.sites = [];
  }
  if (!state.sites.some((s) => Number(s.id) === Number(state.siteId))) {
    let saved = 0;
    if (keepSelection) {
      try { saved = Number(localStorage.getItem('netdiag-site') || 0) || 0; } catch {}
    }
    const valid = state.sites.find((s) => Number(s.id) === saved);
    state.siteId = valid ? Number(valid.id) : (state.sites[0] ? Number(state.sites[0].id) : 1);
  }
  updateSiteSelect();
}

function updateSiteSelect() {
  const sel = document.getElementById('site-select');
  if (!sel) return;
  sel.innerHTML = state.sites.map((s) =>
    `<option value="${s.id}" ${Number(s.id) === Number(state.siteId) ? 'selected' : ''}>${esc(s.name)}</option>`
  ).join('');
  sel.disabled = state.sites.length <= 1;
  const label = document.getElementById('site-hint');
  if (label) label.textContent = currentSite() ? currentSite().name : '';
}

async function switchSite(id) {
  const n = Number(id) || 0;
  if (!n || n === Number(state.siteId)) return;
  state.siteId = n;
  try { localStorage.setItem('netdiag-site', String(n)); } catch {}
  state.hosts.page = 1;
  state.alerts.page = 1;
  updateSiteSelect();
  await route(true);
  refreshAlertBadge();
}

function updateNav() {
  $$('.nav-item').forEach((el) => {
    el.classList.toggle('active', el.dataset.view === state.view);
  });
}

const PAGE_META = {
  dashboard: ['Panel', 'Resumen operativo'],
  hosts: ['Dispositivos', 'Inventario'],
  topology: ['Topología', 'Mapa de red'],
  diagnostics: ['Diagnósticos', 'Problemas empresariales'],
  dashboards: ['Dashboards', 'Vistas personalizadas'],
  wifi: ['WiFi', 'APs, clientes e interferencias'],
  vpn: ['VPN', 'Túneles detectados'],
  alerts: ['Alertas', 'Reglas y canales'],
  scans: ['Escaneos', 'Historial y ejecución'],
  reports: ['Informes', 'Exportación'],
  guide: ['Guía', 'Guía interactiva'],
  settings: ['Configuración', 'Sistema'],
};

function setPageTitle(view, crumb = '') {
  const meta = PAGE_META[view] || ['NetDiag', ''];
  const t = $('#page-title');
  if (t) t.textContent = tr(meta[0]);
  const c = $('#page-crumb');
  if (c) c.textContent = tr(crumb || meta[1] || '');
}

function startClock() {
  if (state.clockTimer) clearInterval(state.clockTimer);
  const tick = () => {
    const el = $('#clock');
    if (el) el.textContent = new Date().toLocaleString(dateLocale(), { dateStyle: 'short', timeStyle: 'medium' });
  };
  tick();
  state.clockTimer = setInterval(tick, 1000);
}

async function refreshAlertBadge() {
  try {
    const stats = await api('/alerts/stats');
    const count = (stats.active || 0) + (stats.acked || 0);
    const el = $('#nav-alert-count');
    if (el) {
      el.textContent = count > 99 ? '99+' : String(count);
      el.classList.toggle('hidden', count === 0);
    }
  } catch {}
}

/* ---------------- websocket ---------------- */
function setLive(on) {
  const dot = $('#live-dot');
  const label = $('#live-label');
  if (dot) dot.classList.toggle('on', on);
  if (label) label.textContent = on ? tr('En vivo') : tr('Reconectando…');
}

function connectWS() {
  if (state.ws) { try { state.ws.close(); } catch {} }
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  let ws;
  try { ws = new WebSocket(`${proto}://${location.host}/api/v1/ws`); } catch { return; }
  state.ws = ws;
  ws.onopen = () => {
    state.wsRetry = 0;
    setLive(true);
    if (state.wsTimer) clearInterval(state.wsTimer);
    state.wsTimer = setInterval(() => {
      if (ws.readyState === 1) { try { ws.send('{"type":"ping"}'); } catch {} }
    }, 25000);
  };
  ws.onclose = () => {
    setLive(false);
    if (state.wsTimer) { clearInterval(state.wsTimer); state.wsTimer = null; }
    if (!state.me) return;
    state.wsRetry = Math.min(state.wsRetry + 1, 6);
    setTimeout(connectWS, Math.min(30000, 800 * Math.pow(1.8, state.wsRetry)));
  };
  ws.onerror = () => { try { ws.close(); } catch {} };
  ws.onmessage = (ev) => {
    let msg;
    try { msg = JSON.parse(ev.data); } catch { return; }
    handleEvent(msg);
  };
}

function scheduleRefresh() {
  if (state.refreshTimer) clearTimeout(state.refreshTimer);
  state.refreshTimer = setTimeout(() => { route(true); }, 900);
}

function handleEvent(ev) {
  const type = ev.type;
  const data = ev.data || {};
  switch (type) {
    case 'hello':
      setLive(true);
      break;
    case 'scan.started':
      toast(`Escaneo de ${data.cidr || 'la red'} iniciado`, { title: 'Escaneo', type: 'ok' });
      if (state.view === 'scans') scheduleRefresh();
      break;
    case 'scan.progress': {
      state.scanActive[data.scan_id] = data;
      const bar = $(`[data-scan-progress="${data.scan_id}"]`);
      if (bar) {
        bar.querySelector('i').style.width = `${Math.min(100, data.percent || 0)}%`;
        const label = bar.parentElement.querySelector('[data-scan-label]');
        if (label) label.textContent = `${data.stage || ''} · ${data.found || 0} encontrados`;
      }
      break;
    }
    case 'scan.finished':
      toast(`${data.cidr || ''}: ${SCAN_LABELS[data.status] || data.status}, ${data.hosts_found || 0} dispositivos`, {
        title: 'Escaneo finalizado', type: data.status === 'error' ? 'err' : 'ok',
      });
      delete state.scanActive[data.id];
      if (['scans', 'dashboard', 'hosts', 'topology'].includes(state.view)) scheduleRefresh();
      break;
    case 'scan.canceled':
      toast('Escaneo cancelado', { type: 'warn' });
      if (state.view === 'scans') scheduleRefresh();
      break;
    case 'host.new':
      toast(`${data.ip}${data.hostname ? ' · ' + data.hostname : ''}`, { title: 'Nuevo dispositivo', type: 'ok' });
      if (['hosts', 'dashboard', 'topology'].includes(state.view)) scheduleRefresh();
      break;
    case 'host.updated':
      if (state.view === 'hosts') scheduleRefresh();
      break;
    case 'host.deleted':
      if (['hosts', 'topology', 'dashboard'].includes(state.view)) scheduleRefresh();
      break;
    case 'alert.new':
      toast(data.message || data.title, {
        title: `Alerta ${SEV_LABELS[data.severity] || ''}: ${data.title || ''}`,
        type: data.severity === 'critical' ? 'err' : data.severity === 'warning' ? 'warn' : '',
      });
      refreshAlertBadge();
      if (state.view === 'alerts') scheduleRefresh();
      break;
    case 'alert.updated':
      refreshAlertBadge();
      if (state.view === 'alerts') scheduleRefresh();
      break;
    case 'report.created':
      toast(data.name || 'Informe generado', { type: 'ok' });
      if (state.view === 'reports') scheduleRefresh();
      break;
    case 'dashboard.updated':
      if (state.view === 'dashboard') scheduleRefresh();
      break;
    case 'sites.updated':
      loadSites(true).then(() => { if (state.view === 'settings') route(true); });
      break;
  }
}

/* ---------------- routing ---------------- */
async function route(silent = false) {
  if (!state.me) return;
  const raw = (location.hash || '#/dashboard').replace(/^#\/?/, '');
  const [view, ...rest] = raw.split('/');
  state.view = ['dashboard', 'hosts', 'topology', 'dashboards', 'diagnostics', 'wifi', 'vpn', 'alerts', 'scans', 'reports', 'guide', 'settings'].includes(view) ? view : 'dashboard';
  if (state.view === 'settings' && !isAdmin()) state.view = 'dashboard';
  updateNav();
  const content = $('#content');
  if (!content) return;
  if (state.view !== 'topology' && !silent) {
    content.animation = 'none';
  }
  try {
    switch (state.view) {
      case 'dashboard': await renderDashboard(); break;
      case 'hosts':
        if (rest[0]) await renderHostDetail(rest[0]);
        else await renderHosts();
        break;
      case 'topology': await renderTopology(); break;
      case 'diagnostics': await renderDiagnostics(); break;
      case 'dashboards': await renderDashboards(); break;
      case 'wifi': await renderWiFi(); break;
      case 'vpn': await renderVPN(); break;
      case 'alerts': await renderAlerts(rest[0]); break;
      case 'scans': await renderScans(); break;
      case 'reports': await renderReports(); break;
      case 'guide': await renderGuide(); break;
      case 'settings': await renderSettings(); break;
      default: await renderDashboard();
    }
  } catch (e) {
    if (e && e.status === 401) return;
    content.innerHTML = `<div class="notice err">${icon('warn')}<div><b>No se pudo cargar la vista</b><div class="tiny">${esc(e.message || e)}</div></div></div>`;
  }
}

function navTo(path) {
  if (location.hash === '#' + path) route();
  else location.hash = path;
}

/* ---------------- dashboard ---------------- */
function statCard(opts) {
  return `<div class="card stat ${opts.tone || ''}">
    <div class="icon">${icon(opts.icon)}</div>
    <div class="label">${esc(opts.label)}</div>
    <div class="value">${opts.value}</div>
    <div class="hint">${opts.hint || ''}</div>
  </div>`;
}

function ring(value, label) {
  const pct = Math.max(0, Math.min(100, value || 0));
  const r = 34, c = 2 * Math.PI * r;
  const off = c * (1 - pct / 100);
  const color = pct >= 99 ? 'var(--green)' : pct >= 95 ? 'var(--amber)' : 'var(--red)';
  return `<div style="display:flex;align-items:center;gap:16px">
    <svg width="86" height="86" viewBox="0 0 86 86">
      <circle cx="43" cy="43" r="${r}" fill="none" stroke="var(--panel-3)" stroke-width="9"/>
      <circle cx="43" cy="43" r="${r}" fill="none" stroke="${color}" stroke-width="9" stroke-linecap="round"
        stroke-dasharray="${c.toFixed(1)}" stroke-dashoffset="${off.toFixed(1)}" transform="rotate(-90 43 43)"/>
      <text x="43" y="47" text-anchor="middle" fill="var(--text)" font-size="17" font-weight="700" font-family="system-ui">${pct.toFixed(1)}%</text>
    </svg>
    <div><div class="label muted tiny" style="text-transform:uppercase;letter-spacing:.08em">${esc(label)}</div>
      <div class="small muted" style="margin-top:4px">Últimas 24 horas</div></div>
  </div>`;
}

async function renderDashboard() {
  setPageTitle('dashboard');
  const content = $('#content');
  content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
  const [dash, scans, alerts, ports] = await Promise.all([
    api('/dashboard'),
    api('/scans?limit=6'),
    api('/alerts?status=active&limit=6'),
    api('/ports/summary'),
  ]);
  state.stats = dash.stats;
  const s = dash.stats || {};
  const byType = Object.entries(s.by_type || {}).sort((a, b) => b[1] - a[1]).slice(0, 7);
  const maxType = Math.max(1, ...byType.map(([, v]) => v));
  const topPorts = (ports.items || []).slice(0, 7);
  const maxPort = Math.max(1, ...topPorts.map((p) => p.hosts));
  const activeScans = scans.active || [];
  const lastScan = s.last_scan;

  content.innerHTML = `
    ${activeScans.length ? `<div class="card pad mb" style="border-color:color-mix(in srgb,var(--accent) 35%,var(--border))">
      <div class="spread"><div class="row">${icon('radar')}<b>Escaneo${activeScans.length > 1 ? 's' : ''} en curso</b>
      <span class="muted small">${activeScans.map((x) => esc(x.cidr)).join(' · ')}</span></div>
      <a class="btn sm" href="#/scans">Ver detalle</a></div></div>` : ''}
    <div class="grid cols-4 mb">
      ${statCard({ label: 'Dispositivos', value: fmtNum(s.hosts_total), hint: `${fmtNum(s.hosts_new_24h)} nuevos en 24 h`, icon: 'hosts', tone: 'accent' })}
      ${statCard({ label: 'En línea', value: fmtNum(s.hosts_up), hint: `${fmtNum(s.hosts_down)} caídos`, icon: 'wifi', tone: 'green' })}
      ${statCard({ label: 'Alertas activas', value: fmtNum(s.alerts_active), hint: `${fmtNum(s.alerts_critical)} críticas`, icon: 'alerts', tone: s.alerts_critical ? 'red' : 'amber' })}
      ${statCard({ label: 'Latencia media', value: `${fmtNum(s.avg_rtt, 1)} <span class="tiny muted">ms</span>`, hint: `${fmtNum(s.scans_24h)} escaneos en 24 h`, icon: 'bolt', tone: 'amber' })}
    </div>
    <div class="grid cols-3 mb">
      <div class="card pad">
        <div class="spread mb"><h3 style="margin:0;font-size:13.5px">Disponibilidad</h3><span class="tiny muted">24 h</span></div>
        ${ring(s.availability || 0, 'Disponibilidad media')}
        <div class="divider"></div>
        <div class="kpi-mini">
          <div><b>${fmtNum(s.hosts_up)}</b><span>en línea</span></div>
          <div><b>${fmtNum(s.hosts_down)}</b><span>caídos</span></div>
          <div><b>${fmtNum(s.networks)}</b><span>redes</span></div>
        </div>
      </div>
      <div class="card pad">
        <div class="spread mb"><h3 style="margin:0;font-size:13.5px">Tipos de dispositivo</h3><a class="tiny" href="#/hosts">Ver todos</a></div>
        ${byType.length ? byType.map(([k, v]) => `<div class="bar-row"><span class="ellipsis">${esc(typeLabel(k))}</span>
          <span class="bar"><i style="width:${(v / maxType) * 100}%"></i></span><b class="right">${v}</b></div>`).join('') : '<div class="empty">Sin datos todavía</div>'}
      </div>
      <div class="card pad">
        <div class="spread mb"><h3 style="margin:0;font-size:13.5px">Puertos más expuestos</h3><a class="tiny" href="#/hosts">Inventario</a></div>
        ${topPorts.length ? topPorts.map((p) => `<div class="bar-row"><span class="mono">${p.port}<span class="muted">/${esc(p.service || '')}</span></span>
          <span class="bar"><i style="width:${(p.hosts / maxPort) * 100}%;background:linear-gradient(90deg,var(--violet),var(--accent))"></i></span><b class="right">${p.hosts}</b></div>`).join('') : '<div class="empty">Sin puertos escaneados</div>'}
      </div>
    </div>
    <div class="grid cols-2">
      <div class="card">
        <div class="card-head"><h3>Alertas activas</h3><a class="tiny" href="#/alerts">Gestionar</a></div>
        ${alerts.items && alerts.items.length ? `<div class="list">${alerts.items.map((a) => `
          <div class="list-item">
            <span class="sev-dot sev-${esc(a.severity)}" style="margin-top:6px"></span>
            <div class="main"><div class="title">${esc(a.title)}</div>
              <div class="meta">${esc(a.message)}</div>
              <div class="meta tiny">${a.host_ip ? `<span class="mono-ip">${esc(a.host_ip)}</span> · ` : ''}${ago(a.created_at)}</div>
            </div>
            <div class="side">${sevBadge(a.severity)}</div>
          </div>`).join('')}</div>` : '<div class="empty">Sin alertas activas</div>'}
      </div>
      <div class="card">
        <div class="card-head"><h3>Escaneos recientes</h3><a class="tiny" href="#/scans">Historial</a></div>
        ${scans.items && scans.items.length ? `<div class="table-wrap"><table><thead><tr><th>Red</th><th>Estado</th><th>Hallazgos</th><th>Inicio</th></tr></thead>
          <tbody>${scans.items.map((x) => `<tr class="clickable" data-action="nav" data-view="scans">
            <td class="mono">${esc(x.cidr)}</td><td>${scanBadge(x.status)}</td><td>${x.hosts_found || 0}</td><td class="muted tiny">${ago(x.started_at || x.finished_at)}</td>
          </tr>`).join('')}</tbody></table></div>` : '<div class="empty">Todavía no se ejecutó ningún escaneo</div>'}
        ${lastScan ? `<div class="card-body tiny muted" style="padding-top:10px">Último: ${esc(lastScan.cidr)} · ${fmtDate(lastScan.finished_at || lastScan.started_at)}</div>` : ''}
      </div>
    </div>`;
}

/* ---------------- hosts list ---------------- */
async function ensureNetworks(force = false) {
  if (!force && state.networks.length) return state.networks;
  const res = await api('/networks');
  state.networks = res.items || [];
  return state.networks;
}

function hostQuery() {
  const q = new URLSearchParams();
  q.set('page', state.hosts.page);
  q.set('limit', state.hosts.limit);
  q.set('sort', state.hosts.sort);
  q.set('asc', state.hosts.asc ? '1' : '0');
  const f = state.hosts.filters;
  if (f.search) q.set('search', f.search);
  if (f.status) q.set('status', f.status);
  if (f.device_type) q.set('device_type', f.device_type);
  if (f.network_id) q.set('network_id', f.network_id);
  if (f.favorite) q.set('favorite', '1');
  return q;
}

async function renderHosts() {
  setPageTitle('hosts');
  const content = $('#content');
  await ensureNetworks();
  const res = await api('/hosts?' + hostQuery().toString());
  state.hosts.items = res.items || [];
  state.hosts.total = res.total || 0;
  const f = state.hosts.filters;
  const pages = Math.max(1, Math.ceil(state.hosts.total / state.hosts.limit));
  const sortIcon = (col) => state.hosts.sort === col ? `<span class="muted">${state.hosts.asc ? '↑' : '↓'}</span>` : '';
  const netOpts = state.networks.map((n) => `<option value="${n.id}" ${String(f.network_id) === String(n.id) ? 'selected' : ''}>${esc(n.name || n.cidr)}</option>`).join('');
  const types = Object.keys(TYPE_LABELS).filter((t) => t !== 'unknown');

  content.innerHTML = `
    <div class="card">
      <div class="filters">
        <div class="search">
          ${icon('search')}
          <input class="input" id="host-search" data-filter="search" placeholder="Buscar IP, nombre, MAC, fabricante…" value="${esc(f.search)}" autocomplete="off">
        </div>
        <select class="select" data-filter="status">
          <option value="">Todos los estados</option>
          ${['up', 'down', 'unknown'].map((s) => `<option value="${s}" ${f.status === s ? 'selected' : ''}>${STATUS_LABELS[s]}</option>`).join('')}
        </select>
        <select class="select" data-filter="device_type">
          <option value="">Todos los tipos</option>
          ${types.map((t) => `<option value="${t}" ${f.device_type === t ? 'selected' : ''}>${typeLabel(t)}</option>`).join('')}
        </select>
        <select class="select" data-filter="network_id">
          <option value="">Todas las redes</option>${netOpts}
        </select>
        <label class="check tiny"><input type="checkbox" data-filter="favorite" ${f.favorite ? 'checked' : ''}> Solo favoritos</label>
        <div class="spacer right"></div>
        <a class="btn sm" href="/api/v1/hosts/export?format=csv" download>${icon('download')} CSV</a>
        <button class="btn sm" data-action="reload">${icon('refresh')} Actualizar</button>
        <button class="btn sm primary" data-action="open-scan">${icon('plus')} Escanear</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr>
            <th class="sortable" data-sort="ip">Dispositivo ${sortIcon('ip')}</th>
            <th class="sortable" data-sort="vendor">Fabricante ${sortIcon('vendor')}</th>
            <th class="sortable" data-sort="device_type">Tipo ${sortIcon('device_type')}</th>
            <th class="sortable" data-sort="status">Estado ${sortIcon('status')}</th>
            <th class="sortable" data-sort="latency_ms">Latencia ${sortIcon('latency_ms')}</th>
            <th>Puertos</th>
            <th class="sortable" data-sort="last_seen">Visto ${sortIcon('last_seen')}</th>
            <th></th>
          </tr></thead>
          <tbody>
          ${state.hosts.items.length ? state.hosts.items.map(hostRow).join('') : `<tr><td colspan="8"><div class="empty">${icon('hosts')}No hay dispositivos con estos filtros</div></td></tr>`}
          </tbody>
        </table>
      </div>
      <div class="pagination">
        <span>${fmtNum(state.hosts.total)} dispositivos · página ${state.hosts.page} de ${pages}</span>
        <div class="spacer right"></div>
        <button class="btn sm" data-action="hosts-page" data-page="${state.hosts.page - 1}" ${state.hosts.page <= 1 ? 'disabled' : ''}>Anterior</button>
        <button class="btn sm" data-action="hosts-page" data-page="${state.hosts.page + 1}" ${state.hosts.page >= pages ? 'disabled' : ''}>Siguiente</button>
      </div>
    </div>`;
}

function hostRow(h) {
  const ports = (h.ports || []).filter((p) => p.state === 'open' || !p.state).slice(0, 3);
  return `<tr class="clickable" data-action="host-open" data-id="${h.id}">
    <td><div class="row"><div class="type-icon">${typeIcon(h.device_type)}</div>
      <div class="name-cell"><b>${esc(h.hostname || h.ip)}</b><span class="mono">${esc(h.ip)}${h.mac ? ' · ' + esc(h.mac) : ''}</span></div></div></td>
    <td class="ellipsis">${esc(h.vendor || '—')}</td>
    <td><span class="badge violet">${esc(typeLabel(h.device_type))}</span></td>
    <td>${statusBadge(h.status)}${h.is_gateway ? ' <span class="tag">Gateway</span>' : ''}</td>
    <td class="mono">${h.latency_ms ? fmtNum(h.latency_ms, 1) + ' ms' : '—'}</td>
    <td>${ports.length ? `<span class="mono small">${ports.map((p) => p.number).join(', ')}</span>${h.open_port_count > 3 ? `<span class="tiny muted"> +${h.open_port_count - 3}</span>` : ''}` : '<span class="muted">—</span>'}</td>
    <td class="muted tiny">${ago(h.last_seen)}</td>
    <td>${h.favorite ? `<span style="color:var(--amber)">${icon('star', '')}</span>` : ''}</td>
  </tr>`;
}

async function reloadHosts() {
  const res = await api('/hosts?' + hostQuery().toString());
  state.hosts.items = res.items || [];
  state.hosts.total = res.total || 0;
  await renderHosts();
}

/* ---------------- host detail ---------------- */
async function renderHostDetail(id, keepTab = true) {
  setPageTitle('hosts', 'Detalle');
  state.currentHostId = Number(id) || null;
  const content = $('#content');
  await ensureNetworks();
  if (!state.host || !state.host.host || String(state.host.host.id) !== String(id)) {
    content.innerHTML = `<div class="loading"><div class="spinner"></div></div>`;
    const res = await api(`/hosts/${id}`);
    state.host = res;
    state.hostTab = 'resumen';
    state.hostHistoryRange = '24h';
  }
  if (!keepTab) state.hostTab = 'resumen';
  const h = state.host.host;
  const tab = state.hostTab;
  let body = '';
  if (tab === 'resumen') body = hostSummaryTab(h, state.host);
  else if (tab === 'historial') body = await hostHistoryTab(h);
  else body = hostEventsTab(state.host.events || []);

  content.innerHTML = `
    <div class="row mb"><button class="btn sm" data-action="nav" data-view="hosts">← Volver al inventario</button>
      <div class="right row">
        <button class="btn sm" data-action="host-fav" data-id="${h.id}">${icon('star')} ${h.favorite ? 'Quitar favorito' : 'Favorito'}</button>
        <button class="btn sm" data-action="host-rescan" data-id="${h.id}">${icon('radar')} Re-escanear</button>
        <button class="btn sm danger" data-action="host-delete" data-id="${h.id}">${icon('trash')} Eliminar</button>
      </div>
    </div>
    <div class="card pad mb">
      <div class="detail-head">
        <div class="big-icon">${typeIcon(h.device_type)}</div>
        <div class="detail-title">
          <h1>${esc(h.hostname || h.ip)}</h1>
          <div class="sub">
            <span class="mono-ip">${esc(h.ip)}</span>
            ${h.mac ? `<span class="mono">${esc(h.mac)}</span>` : ''}
            ${statusBadge(h.status)}
            ${h.is_gateway ? '<span class="badge info">Puerta de enlace</span>' : ''}
            ${h.favorite ? `<span style="color:var(--amber)">${icon('star')}</span>` : ''}
          </div>
        </div>
        <div class="right kpi-mini">
          <div><b>${fmtNum(h.latency_ms, 1)}</b><span>ms</span></div>
          <div><b>${h.open_port_count || (h.ports || []).length}</b><span>puertos</span></div>
          <div><b>${esc(h.confidence || 0)}%</b><span>confianza</span></div>
        </div>
      </div>
    </div>
    <div class="tabs">
      ${[['resumen', 'Resumen'], ['historial', 'Historial'], ['eventos', 'Eventos']].map(([k, label]) =>
        `<div class="tab ${tab === k ? 'active' : ''}" data-action="host-tab" data-tab="${k}">${label}</div>`).join('')}
    </div>
    <div id="host-tab-body">${body}</div>`;
}

function hostSummaryTab(h, detail) {
  const ports = (h.ports || []).filter((p) => p.state === 'open' || !p.state);
  const snmp = h.snmp;
  const neighbors = detail.neighbors || [];
  return `<div class="grid cols-2">
    <div class="card pad">
      <h3 style="margin:0 0 16px;font-size:13.5px">Identidad</h3>
      <dl class="kv">
        <dt>Dirección IP</dt><dd class="mono">${esc(h.ip)}</dd>
        <dt>MAC</dt><dd class="mono">${esc(h.mac || '—')}</dd>
        <dt>Fabricante</dt><dd>${esc(h.vendor || '—')}</dd>
        <dt>Modelo</dt><dd>${esc(h.model || '—')}</dd>
        <dt>Sistema</dt><dd>${esc(h.os || '—')}</dd>
        <dt>Tipo</dt><dd>${esc(typeLabel(h.device_type))} <span class="muted tiny">(${h.confidence || 0}% confianza)</span></dd>
        <dt>Red</dt><dd>${esc((state.networks.find((n) => n.id === h.network_id) || {}).name || h.network_id || '—')}</dd>
        <dt>TTL</dt><dd>${h.ttl || '—'}</dd>
        <dt>Primera vez</dt><dd>${fmtDate(h.first_seen)}</dd>
        <dt>Última vez</dt><dd>${fmtDate(h.last_seen)} <span class="muted tiny">(${ago(h.last_seen)})</span></dd>
        <dt>Etiquetas</dt><dd>${(h.tags || []).length ? h.tags.map((t) => `<span class="tag">${esc(t)}</span>`).join('') : '<span class="muted">Sin etiquetas</span>'}</dd>
      </dl>
      <div class="divider"></div>
      <div class="field"><label>Notas</label>
        <textarea class="input" id="host-notes" placeholder="Notas internas sobre este dispositivo…">${esc(h.notes || '')}</textarea>
      </div>
      <div class="row mt">
        <input class="input" id="host-tags" placeholder="Etiquetas separadas por comas" value="${esc((h.tags || []).join(', '))}">
        <button class="btn" data-action="host-save-meta" data-id="${h.id}">Guardar</button>
      </div>
    </div>
    <div class="stack">
      <div class="card pad mb">
        <h3 style="margin:0 0 12px;font-size:13.5px">Puertos abiertos (${ports.length})</h3>
        ${ports.length ? `<div class="port-grid">${ports.map((p) => `<div class="port-chip">
          <b>${p.number}</b><span>${esc(p.service || '')}${p.product ? ' · ' + esc(p.product) : ''}</span></div>`).join('')}</div>`
          : '<div class="muted small">Sin puertos abiertos detectados</div>'}
        ${(h.evidence || []).length ? `<div class="divider"></div><div class="tiny muted">${h.evidence.map(esc).join(' · ')}</div>` : ''}
      </div>
      ${snmp ? `<div class="card pad mb">
        <h3 style="margin:0 0 12px;font-size:13.5px">SNMP</h3>
        <dl class="kv">
          <dt>Nombre</dt><dd>${esc(snmp.sys_name || '—')}</dd>
          <dt>Descripción</dt><dd>${esc(snmp.sys_descr || '—')}</dd>
          <dt>Ubicación</dt><dd>${esc(snmp.sys_location || '—')}</dd>
          <dt>Contacto</dt><dd>${esc(snmp.sys_contact || '—')}</dd>
          <dt>Uptime</dt><dd>${snmp.uptime_secs ? Math.floor(snmp.uptime_secs / 86400) + ' d ' + Math.floor((snmp.uptime_secs % 86400) / 3600) + ' h' : '—'}</dd>
        </dl>
        ${(snmp.interfaces || []).length ? `<div class="table-wrap mt"><table><thead><tr><th>Interfaz</th><th>Estado</th><th>MAC</th><th>Velocidad</th></tr></thead><tbody>
          ${snmp.interfaces.slice(0, 12).map((i) => `<tr><td>${esc(i.name || i.descr || i.index)}</td><td>${esc(i.status || '—')}</td><td class="mono tiny">${esc(i.mac || '—')}</td><td>${i.speed_mbps ? fmtNum(i.speed_mbps) + ' Mb' : '—'}</td></tr>`).join('')}
        </tbody></table></div>` : ''}
      </div>` : ''}
      <div class="card pad">
        <h3 style="margin:0 0 12px;font-size:13.5px">Vecinos detectados (${neighbors.length})</h3>
        ${neighbors.length ? `<div class="list">${neighbors.map((n) => `<div class="list-item" style="padding-left:0;padding-right:0">
          <span class="badge ${n.protocol === 'lldp' ? 'info' : n.protocol === 'cdp' ? 'violet' : 'unknown'}">${esc(n.protocol.toUpperCase())}</span>
          <div class="main"><div class="title">${esc(n.remote_name || n.remote_ip || n.remote_mac || 'Desconocido')}</div>
            <div class="meta">${n.remote_ip ? `<span class="mono-ip">${esc(n.remote_ip)}</span>` : ''}${n.remote_mac ? ' · <span class="mono">' + esc(n.remote_mac) + '</span>' : ''}${n.local_port ? ' · puerto ' + esc(n.local_port) : ''}</div></div>
          <div class="side">${n.confidence || 0}%</div>
        </div>`).join('')}</div>` : '<div class="muted small">Sin vecinos L2/L3 (requiere SNMP o ARP local)</div>'}
      </div>
    </div>
  </div>`;
}

async function hostHistoryTab(h) {
  const range = state.hostHistoryRange || '24h';
  const hist = await api(`/hosts/${h.id}/history?from=${encodeURIComponent(range)}&points=240`);
  const rtt = hist.rtt || [];
  const up = hist.uptime || [];
  return `<div class="card">
    <div class="card-head"><h3>Historial de disponibilidad</h3>
      <div class="seg">
        ${[['24h', '24 h'], ['7d', '7 días'], ['30d', '30 días']].map(([k, l]) => `<button class="${range === k ? 'active' : ''}" data-action="host-range" data-range="${k}">${l}</button>`).join('')}
      </div>
    </div>
    <div class="card-body">
      <div class="kpi-mini mb">
        <div><b>${fmtNum(hist.availability, 2)}%</b><span>disponibilidad</span></div>
        <div><b>${fmtNum(hist.avg_rtt, 1)} ms</b><span>RTT medio</span></div>
        <div><b>${fmtNum(hist.min_rtt, 1)} ms</b><span>RTT mínimo</span></div>
        <div><b>${fmtNum(hist.max_rtt, 1)} ms</b><span>RTT máximo</span></div>
        <div><b>${fmtNum(hist.samples)}</b><span>muestras</span></div>
      </div>
      <div class="tiny muted mb">Latencia (ms)</div>
      ${lineChart(rtt, { height: 190 })}
      <div class="tiny muted mt mb">Disponibilidad (%)</div>
      ${lineChart(up, { height: 150, color: 'var(--green)', min: 0, max: 100 })}
    </div>
  </div>`;
}

function hostEventsTab(events) {
  if (!events || !events.length) return `<div class="card"><div class="empty">${icon('clock')}Sin eventos registrados</div></div>`;
  return `<div class="card"><div class="list">${events.map((e) => `
    <div class="list-item">
      <span class="badge ${e.kind === 'discovered' ? 'info' : e.kind === 'offline' ? 'down' : 'unknown'}">${esc(e.kind)}</span>
      <div class="main"><div class="title">${esc(e.message)}</div>
        ${e.data ? `<div class="meta tiny mono">${esc(JSON.stringify(e.data))}</div>` : ''}
      </div>
      <div class="side">${fmtDate(e.created_at)}</div>
    </div>`).join('')}</div></div>`;
}

/* ---------------- charts ---------------- */
function lineChart(points, opts = {}) {
  const height = opts.height || 190, width = 760;
  const pad = { l: 40, r: 12, t: 12, b: 22 };
  if (!points || !points.length) {
    return `<div class="empty" style="padding:30px">Sin datos en el periodo seleccionado</div>`;
  }
  const vals = points.map((p) => Number(p.value) || 0);
  let min = opts.min !== undefined ? opts.min : Math.min(...vals);
  let max = opts.max !== undefined ? opts.max : Math.max(...vals);
  if (min === max) { min = Math.max(0, min - 1); max = max + 1; }
  const n = points.length;
  const X = (i) => pad.l + (n === 1 ? 0 : (width - pad.l - pad.r) * i / (n - 1));
  const Y = (v) => pad.t + (height - pad.t - pad.b) * (1 - (v - min) / (max - min));
  let d = '';
  points.forEach((p, i) => { d += `${i ? 'L' : 'M'}${X(i).toFixed(1)} ${Y(Number(p.value) || 0).toFixed(1)} `; });
  const area = `${d}L${X(n - 1).toFixed(1)} ${height - pad.b} L${X(0).toFixed(1)} ${height - pad.b} Z`;
  const color = opts.color || 'var(--accent)';
  const gid = 'grad' + Math.random().toString(36).slice(2, 8);
  let grid = '';
  for (let i = 0; i <= 4; i++) {
    const y = pad.t + (height - pad.t - pad.b) * i / 4;
    const val = max - (max - min) * i / 4;
    grid += `<line class="grid-line" x1="${pad.l}" y1="${y.toFixed(1)}" x2="${width - pad.r}" y2="${y.toFixed(1)}"/>`;
    grid += `<text class="axis" x="4" y="${(y + 3).toFixed(1)}">${val >= 100 ? Math.round(val) : val.toFixed(1)}</text>`;
  }
  const first = points[0], last = points[n - 1];
  return `<svg class="chart" viewBox="0 0 ${width} ${height}" preserveAspectRatio="none">
    <defs><linearGradient id="${gid}" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0" stop-color="${color}" stop-opacity=".45"/>
      <stop offset="1" stop-color="${color}" stop-opacity="0"/>
    </linearGradient></defs>
    ${grid}
    <path d="${area}" fill="url(#${gid})"/>
    <path d="${d}" fill="none" stroke="${color}" stroke-width="2" stroke-linejoin="round"/>
    <text class="axis" x="${pad.l}" y="${height - 5}">${esc(fmtDate(first.ts, false))}</text>
    <text class="axis" x="${width - pad.r}" y="${height - 5}" text-anchor="end">${esc(fmtTime(last.ts))}</text>
  </svg>`;
}

/* ---------------- global event delegation ---------------- */
document.addEventListener('click', async (e) => {
  const target = e.target.closest('[data-action]');
  if (!target) return;
  const action = target.dataset.action;
  if (action === 'nav') {
    e.preventDefault();
    navTo('#/' + target.dataset.view);
    return;
  }
  if (['host-open'].includes(action)) e.preventDefault();
  try {
    switch (action) {
      case 'modal-close': closeModal(); break;
      case 'reload': route(); break;
      case 'refresh-diagnostics': await renderDiagnostics(); break;
      case 'dash-create':
        try {
          const name = (document.getElementById('dash-name') || {}).value || '';
          const created = await api('/dashboards', { method: 'POST', body: { name, site_id: state.siteId || 1, widgets: [] } });
          state.dashboardId = created.id;
          toast(t('Dashboard creado'), { type: 'ok' });
          await renderDashboards();
        } catch (err) { toast(err.message || t('No se pudo crear'), { type: 'err' }); }
        break;
      case 'dash-example':
        try {
          const created = await api('/dashboards/example', { method: 'POST', body: { site_id: state.siteId || 1 } });
          state.dashboardId = created.id;
          toast(`${t('Dashboard creado')}: ${created.name} (${created.widgets.length} ${t('widgets')})`, { type: 'ok' });
          await renderDashboards();
        } catch (err) { toast(err.message || t('No se pudo crear'), { type: 'err' }); }
        break;
      case 'dash-delete':
        if (!confirm(t('¿Eliminar este dashboard?'))) break;
        try {
          await api('/dashboards/' + target.dataset.id, { method: 'DELETE' });
          state.dashboardId = null;
          toast(t('Dashboard eliminado'), { type: 'ok' });
          await renderDashboards();
        } catch (err) { toast(err.message || t('No se pudo eliminar'), { type: 'err' }); }
        break;
      case 'dash-add-widget':
        try {
          const current = await api('/dashboards/' + state.dashboardId);
          const typeSel = document.getElementById('dash-widget-type');
          const type = typeSel ? typeSel.value : 'hosts';
          const widgets = (current.widgets || []).map((w) => Object.assign({}, w));
          const idx = widgets.length;
          widgets.push({
            id: 'w' + Date.now(), type,
            position: { x: (idx % 2) * 6, y: Math.floor(idx / 2) },
            size: { w: 6, h: 1 }, config: {},
          });
          await api('/dashboards/' + state.dashboardId, { method: 'PUT', body: { name: current.name, site_id: current.site_id, widgets } });
          await renderDashboards();
        } catch (err) { toast(err.message || t('No se pudo guardar'), { type: 'err' }); }
        break;
      case 'dash-remove-widget':
        try {
          const current = await api('/dashboards/' + state.dashboardId);
          const widgets = (current.widgets || []).filter((w) => w.id !== target.dataset.id)
            .map((w, idx) => Object.assign({}, w, { position: { x: (idx % 2) * 6, y: Math.floor(idx / 2) } }));
          await api('/dashboards/' + state.dashboardId, { method: 'PUT', body: { name: current.name, site_id: current.site_id, widgets } });
          await renderDashboards();
        } catch (err) { toast(err.message || t('No se pudo guardar'), { type: 'err' }); }
        break;
      case 'wifi-scan':
        try {
          const r = await api('/wifi/scan', { method: 'POST' });
          toast(`${r.result.aps} ${t('APs observados')} · ${r.result.clients} ${t('clientes observados')}`, { type: 'ok' });
          await renderWiFi();
        } catch (err) { toast(err.message || t('No se pudo capturar'), { type: 'err' }); }
        break;
      case 'vpn-scan':
        try {
          const r = await api('/vpn/scan', { method: 'POST' });
          toast(`${r.stored} ${t('túneles observados')}`, { type: 'ok' });
          await renderVPN();
        } catch (err) { toast(err.message || t('No se pudo observar'), { type: 'err' }); }
        break;
      case 'logout': logout(); break;
      case 'theme': toggleTheme(); break;
      case 'lang': if (window.NetDiagI18n) window.NetDiagI18n.toggle(); break;
      case 'guide-open': navTo('#/guide'); break;
      case 'tour-start': startTour(); break;
      case 'tour-next': tourNext(); break;
      case 'tour-prev': tourPrev(); break;
      case 'tour-end': endTour(); break;
      case 'user-reset': openResetUserModal(Number(target.dataset.id), target.dataset.name || ''); break;
      case 'user-password-save': await submitUserPasswordFromModal(); break;
      case 'user-delete': await deleteUserAccount(Number(target.dataset.id), target.dataset.name || '', target.dataset.self === '1'); break;
      case 'backup-create': await createBackup(target); break;
      case 'backup-delete': await deleteBackup(target.dataset.name || ''); break;
      case 'toggle-sidebar': $('#sidebar')?.classList.toggle('open'); break;
      case 'open-scan': openScanModal(); break;
      case 'hosts-page':
        state.hosts.page = Math.max(1, Number(target.dataset.page) || 1);
        await renderHosts();
        break;
      case 'host-open':
        navTo('#/hosts/' + target.dataset.id);
        break;
      case 'host-tab':
        state.hostTab = target.dataset.tab;
        await renderHostDetail(state.host.host.id);
        break;
      case 'host-range':
        state.hostHistoryRange = target.dataset.range;
        await renderHostDetail(state.host.host.id);
        break;
      case 'host-fav': await toggleFavorite(Number(target.dataset.id)); break;
      case 'host-rescan': await rescanHost(Number(target.dataset.id)); break;
      case 'host-delete': await deleteHost(Number(target.dataset.id)); break;
      case 'host-save-meta': await saveHostMeta(Number(target.dataset.id)); break;
      case 'scan-cancel': await cancelScan(Number(target.dataset.id)); break;
      case 'alerts-tab':
        await renderAlerts(target.dataset.tab || 'alertas');
        break;
      case 'alerts-page':
        state.alerts.page = Math.max(1, Number(target.dataset.page) || 1);
        await renderAlerts();
        break;
      case 'alert-ack': await setAlertStatus(Number(target.dataset.id), 'ack'); break;
      case 'alert-resolve': await setAlertStatus(Number(target.dataset.id), 'resolve'); break;
      case 'rule-new': openRuleModal(); break;
      case 'rule-edit': openRuleModal(Number(target.dataset.id)); break;
      case 'rule-save': await saveRuleFromModal(Number(target.dataset.id) || 0); break;
      case 'rule-delete': await deleteRule(Number(target.dataset.id)); break;
      case 'rule-toggle': await toggleRule(Number(target.dataset.id), target.dataset.enabled !== 'true'); break;
      case 'channel-new': openChannelModal(); break;
      case 'channel-edit': openChannelModal(Number(target.dataset.id)); break;
      case 'channel-save': await saveChannelFromModal(Number(target.dataset.id) || 0); break;
      case 'channel-toggle': await toggleChannel(Number(target.dataset.id), target.dataset.enabled !== 'true'); break;
      case 'channel-delete': await deleteChannel(Number(target.dataset.id)); break;
      case 'channel-test': await testChannel(Number(target.dataset.id), target); break;
      case 'integration-new': openIntegrationModal(); break;
      case 'integration-edit': openIntegrationModal(Number(target.dataset.id)); break;
      case 'integration-save': await saveIntegrationFromModal(Number(target.dataset.id) || 0); break;
      case 'integration-toggle': await toggleIntegration(Number(target.dataset.id), target.dataset.enabled === 'true'); break;
      case 'integration-delete': await deleteIntegration(Number(target.dataset.id)); break;
      case 'integration-test': await testIntegration(Number(target.dataset.id), target); break;
      case 'report-new': openReportModal(); break;
      case 'report-create': await createReportFromModal(); break;
      case 'report-delete': await deleteReport(Number(target.dataset.id)); break;
      case 'scan-start': await startScanFromModal(); break;
      case 'settings-tab': state.settingsTab = target.dataset.tab; await renderSettings(); break;
      case 'site-new': openSiteModal(); break;
      case 'site-edit': openSiteModal(Number(target.dataset.id)); break;
      case 'site-save': await saveSiteFromModal(Number(target.dataset.id) || 0); break;
      case 'site-delete': await deleteSite(Number(target.dataset.id)); break;
      case 'site-activate': await switchSite(Number(target.dataset.id)); if (state.view === 'settings') await renderSettings(); break;
      case 'site-export': await exportSite(Number(target.dataset.id)); break;
      case 'site-import': openImportSiteModal(Number(target.dataset.id)); break;
      case 'site-import-run': {
        const form = document.querySelector('form[data-form="site-import"]');
        if (form) await submitSiteImport(form);
        break;
      }
      case 'network-new': openNetworkModal(); break;
      case 'network-edit': openNetworkModal(Number(target.dataset.id)); break;
      case 'network-save': await saveNetworkFromModal(Number(target.dataset.id) || 0); break;
      case 'network-delete': await deleteNetwork(Number(target.dataset.id)); break;
      case 'network-suggest': await addSuggestedNetwork(target.dataset.cidr, target.dataset.iface); break;
      case 'user-new': openUserModal(); break;
      case 'user-save': await saveUserFromModal(); break;
      case 'topo-zoom': zoomTopology(Number(target.dataset.delta)); break;
      case 'topo-reset': resetTopology(); break;
      case 'copy-text': await copyText(target.dataset.text); break;
      case 'reload-app': location.reload(); break;
      case 'reauth': logout(); break;
    }
  } catch (err) {
    if (err && err.status === 401) return;
    toast(err.message || String(err), { type: 'err', title: 'Error' });
  }
});

document.addEventListener('change', (e) => {
  const el = e.target;
  if (el.id === 'site-select') {
    switchSite(el.value).catch((err) => toast(err.message || String(err), { type: 'err', title: 'Error' }));
    return;
  }
  if (el.dataset.action === 'dash-select') {
    state.dashboardId = Number(el.value);
    renderDashboards().catch((err) => toast(err.message || String(err), { type: 'err', title: 'Error' }));
    return;
  }
  if (el.dataset.filter !== undefined) {
    const key = el.dataset.filter;
    if (el.type === 'checkbox') state.hosts.filters[key] = el.checked;
    else state.hosts.filters[key] = el.value;
    state.hosts.page = 1;
    debouncedHostsReload();
  }
  if (el.dataset.alertsFilter !== undefined) {
    state.alerts[el.dataset.alertsFilter] = el.value;
    state.alerts.page = 1;
    renderAlerts(state.alertsTab || 'alertas');
  }
  if (el.dataset.hostRange) {
    state.hostHistoryRange = el.value;
    renderHostDetail(state.host.host.id);
  }
  if (el.dataset.userRole !== undefined) {
    changeUserRole(Number(el.dataset.userRole), el.value).catch((err) => toast(err.message || String(err), { type: 'err', title: 'Error' }));
  }
});

const debouncedHostsReload = debounce(() => { if (state.view === 'hosts') reloadHosts(); }, 320);

document.addEventListener('click', async (e) => {
  const th = e.target.closest('th.sortable[data-sort]');
  if (!th || state.view !== 'hosts') return;
  const col = th.dataset.sort;
  if (state.hosts.sort === col) state.hosts.asc = !state.hosts.asc;
  else { state.hosts.sort = col; state.hosts.asc = col !== 'last_seen'; }
  state.hosts.page = 1;
  await reloadHosts();
});

document.addEventListener('input', (e) => {
  if (e.target.dataset.filter === 'search') {
    state.hosts.filters.search = e.target.value;
    state.hosts.page = 1;
    debouncedHostsReload();
  }
  if (e.target.dataset.alertsSearch !== undefined) {
    state.alerts.search = e.target.value;
    debouncedAlertsReload();
  }
  if (e.target.dataset.guideSearch !== undefined && typeof filterGuide === 'function') {
    filterGuide(e.target.value);
  }
});

const debouncedAlertsReload = debounce(() => { if (state.view === 'alerts') renderAlerts(state.alertsTab || 'alertas'); }, 320);

document.addEventListener('submit', async (e) => {
  const form = e.target.closest('form[data-form]');
  if (!form) return;
  e.preventDefault();
  const kind = form.dataset.form;
  try {
    if (kind === 'login' || kind === 'setup') {
      await submitAuth(form);
      return;
    }
    if (kind === 'scan') { await submitScan(form); return; }
    if (kind === 'rule') { await submitRule(form); return; }
    if (kind === 'channel') { await submitChannel(form); return; }
    if (kind === 'integration') { await submitIntegration(form); return; }
    if (kind === 'network') { await submitNetwork(form); return; }
    if (kind === 'user') { await submitUser(form); return; }
    if (kind === 'reset-password') { await submitUserPassword(form); return; }
    if (kind === 'report') { await submitReport(form); return; }
    if (kind === 'config') { await submitConfig(form); return; }
    if (kind === 'password') { await submitPassword(form); return; }
  } catch (err) {
    if (err && err.status === 401) return;
    toast(err.message || String(err), { type: 'err', title: 'Error' });
    const btn = form.querySelector('button[type=submit]');
    if (btn) btn.disabled = false;
  }
});

const THEMES = ['dark', 'light', 'high-contrast'];

function themeIcon(theme) {
  return theme === 'dark' ? 'sun' : theme === 'light' ? 'moon' : 'contrast';
}

function themeLabel(theme) {
  return t(theme === 'dark' ? 'Tema oscuro' : theme === 'light' ? 'Tema claro' : 'Alto contraste');
}

function applyTheme(theme, persist = true) {
  const next = THEMES.includes(theme) ? theme : 'dark';
  document.documentElement.dataset.theme = next;
  if (persist) {
    try { localStorage.setItem('netdiag-theme', next); } catch {}
  }
  const btn = $('#theme-btn');
  if (btn) {
    btn.innerHTML = icon(themeIcon(next));
    btn.title = t('Cambiar tema') + ': ' + themeLabel(next);
  }
}

function toggleTheme() {
  const current = THEMES.indexOf(document.documentElement.dataset.theme || 'dark');
  applyTheme(THEMES[(current + 1) % THEMES.length]);
}

async function copyText(text) {
  try { await navigator.clipboard.writeText(text); toast('Copiado al portapapeles', { type: 'ok' }); }
  catch { toast('No se pudo copiar', { type: 'warn' }); }
}

/* ---------------- host actions ---------------- */
async function toggleFavorite(id) {
  const h = state.host && state.host.host;
  const next = !(h && h.id === id && h.favorite);
  await api(`/hosts/${id}`, { method: 'PATCH', body: { favorite: next } });
  await api(`/hosts/${id}`).then((res) => { state.host = res; });
  await renderHostDetail(id);
  toast(next ? 'Añadido a favoritos' : 'Quitado de favoritos', { type: 'ok' });
}

async function rescanHost(id) {
  const res = await api(`/hosts/${id}/rescan`, { method: 'POST', body: { profile: 'full' } });
  toast(`Escaneo #${res.id} programado`, { type: 'ok' });
}

async function deleteHost(id) {
  if (!confirm('¿Eliminar este dispositivo del inventario? Se conservarán sus métricas históricas solo si vuelve a aparecer.')) return;
  await api(`/hosts/${id}`, { method: 'DELETE' });
  toast('Dispositivo eliminado', { type: 'ok' });
  navTo('#/hosts');
  state.host = null;
}

async function saveHostMeta(id) {
  const notes = $('#host-notes')?.value || '';
  const tagsRaw = $('#host-tags')?.value || '';
  const tags = tagsRaw.split(',').map((t) => t.trim()).filter(Boolean);
  const updated = await api(`/hosts/${id}`, { method: 'PATCH', body: { notes, tags } });
  state.host.host = updated;
  await renderHostDetail(id);
  toast('Cambios guardados', { type: 'ok' });
}

/* ---------------- alerts helpers (used by views.js) ---------------- */
async function setAlertStatus(id, action) {
  await api(`/alerts/${id}/${action}`, { method: 'POST' });
  toast(action === 'ack' ? 'Alerta reconocida' : 'Alerta resuelta', { type: 'ok' });
  refreshAlertBadge();
  if (state.view === 'alerts') await renderAlerts(state.alertsTab || 'alertas');
}

/* ---------------- guía interactiva / tour ---------------- */
const TOUR_STEPS = [
  {
    selector: '[data-view="dashboard"]', title: 'Panel', titleEn: 'Dashboard',
    body: 'Aquí ves el estado de la red: dispositivos, alertas, latencia y disponibilidad de las últimas 24 horas.',
    bodyEn: 'The state of your network at a glance: devices, alerts, latency and availability for the last 24 hours.',
  },
  {
    selector: '[data-view="hosts"]', title: 'Dispositivos', titleEn: 'Devices',
    body: 'El inventario guarda cada host con su IP, MAC, fabricante, puertos, notas y etiquetas. La identidad se conserva aunque cambie la IP.',
    bodyEn: 'The inventory keeps every host with its IP, MAC, vendor, ports, notes and tags. Identity survives IP changes.',
  },
  {
    selector: '[data-view="topology"]', title: 'Topología', titleEn: 'Topology',
    body: 'Mapa interactivo de enlaces L2/L3 construido con LLDP, CDP, tablas de bridge y ARP. Arrastra, acerca y filtra por subred.',
    bodyEn: 'Interactive L2/L3 link map built from LLDP, CDP, bridge tables and ARP. Drag, zoom and filter by subnet.',
  },
  {
    selector: '[data-view="alerts"]', title: 'Alertas', titleEn: 'Alerts',
    body: 'Reglas integradas y personalizadas. Reconoce, resuelve y envía notificaciones por webhook, Slack, Telegram, correo o Syslog.',
    bodyEn: 'Built-in and custom rules. Acknowledge, resolve, and notify through webhooks, Slack, Telegram, email or Syslog.',
  },
  {
    selector: '[data-view="scans"]', title: 'Escaneos', titleEn: 'Scans',
    body: 'Lanza descubrimientos por perfil y sigue el progreso en vivo. Puedes cancelar sin perder lo ya descubierto.',
    bodyEn: 'Run discoveries by profile and follow progress live. Cancel at any time without losing what was already found.',
  },
  {
    selector: '[data-view="reports"]', title: 'Informes', titleEn: 'Reports',
    body: 'Genera inventarios, resúmenes ejecutivos y listados de alertas en PDF, CSV, JSON o HTML.',
    bodyEn: 'Generate inventories, executive summaries and alert lists in PDF, CSV, JSON or HTML.',
  },
  {
    selector: '[data-view="guide"]', title: 'Guía', titleEn: 'Guide',
    body: 'La guía completa vive aquí. Puedes repetir este recorrido cuando quieras.',
    bodyEn: 'The full guide lives here. You can replay this tour whenever you like.',
  },
  {
    selector: '[data-action="lang"]', title: 'Idioma', titleEn: 'Language',
    body: 'Cambia entre español e inglés en cualquier momento. La preferencia se guarda en este navegador.',
    bodyEn: 'Switch between Spanish and English at any time. The preference is stored in this browser.',
  },
  {
    selector: '[data-view="settings"]', title: 'Configuración', titleEn: 'Settings',
    body: 'Redes, usuarios, copias de seguridad y ajustes avanzados. Solo los administradores ven esta sección.',
    bodyEn: 'Networks, users, backups and advanced options. Only administrators see this section.',
    adminOnly: true,
  },
];

let tourState = null;

function startTour() {
  if (!state.me) return;
  const steps = TOUR_STEPS.filter((s) => {
    if (s.adminOnly && !isAdmin()) return false;
    const el = document.querySelector(s.selector);
    return el && (el.offsetWidth || el.offsetHeight || el.getClientRects().length);
  });
  if (!steps.length) return;
  closeModal();
  tourState = { index: 0, steps };
  renderTour();
}

function tourText(step, key) {
  const en = window.NetDiagI18n && window.NetDiagI18n.isEnglish;
  if (en && step[key + 'En']) return step[key + 'En'];
  return step[key];
}

function tourRoot() {
  let root = document.getElementById('tour-root');
  if (!root) {
    root = document.createElement('div');
    root.id = 'tour-root';
    root.className = 'tour-layer';
    document.body.appendChild(root);
  }
  return root;
}

function renderTour() {
  if (!tourState) return;
  const step = tourState.steps[tourState.index];
  if (!step) return endTour();
  const root = tourRoot();
  const last = tourState.index === tourState.steps.length - 1;
  root.innerHTML = `
    <div class="tour-spot" id="tour-spot"></div>
    <div class="tour-pop" id="tour-pop" role="dialog" aria-modal="true">
      <div class="tour-progress"><span>${tr('Paso')} ${tourState.index + 1} / ${tourState.steps.length}</span>
        <button class="btn ghost icon sm" data-action="tour-end" aria-label="${esc(tr('Cerrar'))}">${icon('x')}</button></div>
      <h3>${esc(tourText(step, 'title'))}</h3>
      <p>${esc(tourText(step, 'body'))}</p>
      <div class="tour-actions">
        <button class="btn ghost" data-action="tour-end">${esc(tr('Saltar'))}</button>
        <div class="spacer"></div>
        ${tourState.index > 0 ? `<button class="btn" data-action="tour-prev">${esc(tr('Anterior'))}</button>` : ''}
        <button class="btn primary" data-action="tour-next">${esc(last ? tr('Finalizar') : tr('Siguiente'))}</button>
      </div>
    </div>`;
  positionTour(step);
  const pop = document.getElementById('tour-pop');
  if (pop) pop.focus();
}

function positionTour(step) {
  const target = document.querySelector(step.selector);
  const spot = document.getElementById('tour-spot');
  const pop = document.getElementById('tour-pop');
  if (!target || !spot || !pop) return;
  target.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  const r = target.getBoundingClientRect();
  const pad = 6;
  spot.style.top = `${r.top - pad}px`;
  spot.style.left = `${r.left - pad}px`;
  spot.style.width = `${r.width + pad * 2}px`;
  spot.style.height = `${r.height + pad * 2}px`;

  const pw = Math.min(340, window.innerWidth - 32);
  pop.style.width = `${pw}px`;
  const rect = pop.getBoundingClientRect();
  const ph = rect.height || 220;
  let top = r.bottom + 14;
  if (top + ph > window.innerHeight - 12) top = Math.max(12, r.top - ph - 14);
  let left = r.left + r.width / 2 - pw / 2;
  left = Math.max(12, Math.min(window.innerWidth - pw - 12, left));
  pop.style.top = `${top}px`;
  pop.style.left = `${left}px`;
}

function tourNext() {
  if (!tourState) return;
  if (tourState.index >= tourState.steps.length - 1) return endTour();
  tourState.index++;
  renderTour();
}

function tourPrev() {
  if (!tourState || tourState.index === 0) return;
  tourState.index--;
  renderTour();
}

function endTour() {
  tourState = null;
  try { localStorage.setItem('netdiag-tour-done', '1'); } catch {}
  const root = document.getElementById('tour-root');
  if (root) root.remove();
}

window.addEventListener('resize', () => {
  if (tourState) {
    const step = tourState.steps[tourState.index];
    if (step) positionTour(step);
  }
});

/* ---------------- boot ---------------- */
async function boot() {
  let saved = null;
  try { saved = localStorage.getItem('netdiag-theme'); } catch {}
  if (saved && THEMES.includes(saved)) {
    document.documentElement.dataset.theme = saved;
  } else if (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches) {
    document.documentElement.dataset.theme = 'light';
  }
  try {
    const st = await api('/auth/status', { noAuthRedirect: true });
    state.auth = st;
    if (st.setup_required) { showSetup(st); return; }
    if (st.auth_enabled && !st.authenticated) { showLogin(st); return; }
    state.me = st.user || { username: 'local', role: 'admin', id: 0 };
    state.csrf = st.csrf || '';
    enterApp();
  } catch (e) {
    renderBootError(e);
  }
}

window.addEventListener('hashchange', () => route());
if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
else boot();
