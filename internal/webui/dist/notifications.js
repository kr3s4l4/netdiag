'use strict';

/* ============================================================
 * NetDiag SPA — F30: notification centre.
 * Keeps the important websocket events after a reload. Storage is
 * local to the browser, scoped by site and capped at 100 items.
 * No outbound calls, no external dependencies.
 * Depends on helpers declared in app.js: state, tr, esc, icon,
 * navTo, ago and dateLocale.
 * ============================================================ */

const NOTIF_STORAGE_KEY = 'netdiag-notifications';
const NOTIF_MAX = 100;
const NOTIF_TYPES = new Set([
  'scan.finished', 'alert.new', 'update.available',
  'update.applied', 'update.installed', 'update.rolled_back',
]);

let notifPanelOpen = false;
let notifSite = '';

function notifSiteKey() {
  return notifSite || String(state.siteId || 0);
}

function readNotifStore() {
  try {
    const raw = localStorage.getItem(NOTIF_STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

function loadNotifications() {
  const store = readNotifStore();
  const list = store[notifSiteKey()];
  if (!Array.isArray(list)) return [];
  return list.filter((n) => n && typeof n === 'object' && Number.isFinite(Number(n.ts)))
    .slice(0, NOTIF_MAX);
}

function saveNotifications(list) {
  const store = readNotifStore();
  store[notifSiteKey()] = (Array.isArray(list) ? list : []).slice(0, NOTIF_MAX);
  try { localStorage.setItem(NOTIF_STORAGE_KEY, JSON.stringify(store)); } catch {}
}

function notificationFor(ev) {
  const data = ev.data || {};
  switch (ev.type) {
    case 'scan.finished': {
      const label = (typeof SCAN_LABELS !== 'undefined' && SCAN_LABELS[data.status]) || data.status || '';
      return {
        title: tr('Escaneo finalizado'),
        message: [data.cidr, label, `${data.hosts_found || 0} ${tr('dispositivos')}`].filter(Boolean).join(' · '),
        target: '#/scans',
        tone: data.status === 'error' ? 'err' : 'ok',
      };
    }
    case 'alert.new': {
      const sev = (typeof SEV_LABELS !== 'undefined' && SEV_LABELS[data.severity]) || data.severity || '';
      return {
        title: [tr('Alerta'), sev].filter(Boolean).join(' '),
        message: [data.title, data.message].filter(Boolean).join(' · '),
        target: data.host_id
          ? '#/hosts/' + encodeURIComponent(data.host_id)
          : '#/alerts?q=' + encodeURIComponent(data.title || '') + '&status=all',
        tone: data.severity === 'critical' ? 'err' : data.severity === 'warning' ? 'warn' : '',
      };
    }
    case 'update.available':
      return {
        title: tr('Actualización disponible'),
        message: String(data.latest_version || ''),
        target: '#/settings',
        tone: 'warn',
      };
    case 'update.applied':
    case 'update.installed':
      return {
        title: tr('Actualización instalada'),
        message: String(data.to_version || ''),
        target: '#/settings',
        tone: 'ok',
      };
    case 'update.rolled_back':
      return {
        title: tr('Actualización revertida'),
        message: '',
        target: '#/settings',
        tone: 'warn',
      };
  }
  return null;
}

function recordNotification(ev) {
  if (!ev || !NOTIF_TYPES.has(ev.type) || !state.me) return;
  const info = notificationFor(ev);
  if (!info) return;
  const item = {
    id: Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8),
    ts: Date.now(),
    type: ev.type,
    title: String(info.title || '').slice(0, 160),
    message: String(info.message || '').slice(0, 240),
    target: String(info.target || '#/dashboard').slice(0, 200),
    tone: String(info.tone || '').slice(0, 16),
    read: false,
  };
  const list = loadNotifications();
  list.unshift(item);
  saveNotifications(list);
  refreshNotificationBadge();
  if (notifPanelOpen) renderNotificationPanel();
}

function refreshNotificationBadge() {
  const el = document.getElementById('notif-count');
  if (!el) return;
  const unread = loadNotifications().filter((n) => !n.read).length;
  el.textContent = unread > 99 ? '99+' : String(unread);
  el.classList.toggle('hidden', unread === 0);
  const bell = document.getElementById('notif-bell');
  if (bell) {
    bell.classList.toggle('has-unread', unread > 0);
    bell.setAttribute('aria-expanded', notifPanelOpen ? 'true' : 'false');
  }
}

function notifPanelRoot() {
  let root = document.getElementById('notif-panel-root');
  if (!root) {
    root = document.createElement('div');
    root.id = 'notif-panel-root';
    document.body.appendChild(root);
  }
  return root;
}

function closeNotificationPanel() {
  notifPanelOpen = false;
  const root = document.getElementById('notif-panel-root');
  if (root) root.innerHTML = '';
  refreshNotificationBadge();
}

function notificationRows(list) {
  if (!list.length) return `<div class="notif-empty">${esc(tr('Sin notificaciones'))}</div>`;
  return list.map((n) => `<button class="notif-item ${n.read ? '' : 'unread'}" data-action="notif-open" data-id="${esc(n.id)}">
    <span class="notif-dot ${esc(n.tone || '')}"></span>
    <span class="notif-body"><b>${esc(n.title)}</b><small>${esc(n.message)}</small><time>${esc(ago(n.ts))}</time></span>
  </button>`).join('');
}

function renderNotificationPanel() {
  const list = loadNotifications();
  const unread = list.filter((n) => !n.read).length;
  notifPanelRoot().innerHTML = `<div class="notif-panel" role="dialog" aria-label="${esc(tr('Notificaciones'))}">
    <div class="notif-head">
      <b>${esc(tr('Notificaciones'))}</b>
      <span class="tag">${unread} ${esc(tr('sin leer'))}</span>
      <button class="btn ghost icon sm" data-action="notif-close" aria-label="${esc(tr('Cerrar'))}">${icon('x')}</button>
    </div>
    <div class="notif-list">${notificationRows(list)}</div>
    <div class="notif-foot">
      <button class="btn sm" data-action="notif-read-all" ${unread ? '' : 'disabled'}>${esc(tr('Marcar todo como leído'))}</button>
      <button class="btn sm ghost" data-action="notif-clear" ${list.length ? '' : 'disabled'}>${esc(tr('Borrar historial'))}</button>
    </div>
  </div>`;
}

function toggleNotificationPanel() {
  if (notifPanelOpen) { closeNotificationPanel(); return; }
  notifPanelOpen = true;
  renderNotificationPanel();
  refreshNotificationBadge();
}

document.addEventListener('click', (e) => {
  if (!e.target || !e.target.closest) return;
  if (e.target.closest('[data-action="notif-toggle"]')) {
    e.preventDefault();
    toggleNotificationPanel();
    return;
  }
  if (!notifPanelOpen) return;
  const target = e.target.closest('[data-action]');
  const action = target ? target.dataset.action : '';
  if (action === 'notif-close') { closeNotificationPanel(); return; }
  if (action === 'notif-read-all') {
    saveNotifications(loadNotifications().map((n) => Object.assign({}, n, { read: true })));
    refreshNotificationBadge();
    renderNotificationPanel();
    return;
  }
  if (action === 'notif-clear') {
    saveNotifications([]);
    refreshNotificationBadge();
    renderNotificationPanel();
    return;
  }
  if (action === 'notif-open') {
    const list = loadNotifications();
    const item = list.find((n) => n.id === target.dataset.id);
    if (item) {
      item.read = true;
      saveNotifications(list);
      refreshNotificationBadge();
      if (item.target) navTo(item.target);
    }
    closeNotificationPanel();
    return;
  }
  if (!e.target.closest('.notif-panel')) closeNotificationPanel();
});

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && notifPanelOpen) closeNotificationPanel();
});

document.addEventListener('change', (e) => {
  if (!e.target || e.target.id !== 'site-select') return;
  notifSite = String(Number(e.target.value) || 0);
  refreshNotificationBadge();
  if (notifPanelOpen) renderNotificationPanel();
});

window.NetDiagNotifications = {
  record: recordNotification,
  refresh: refreshNotificationBadge,
};
