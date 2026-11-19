'use strict';

/* ============================================================
 * NetDiag SPA — F27: asistente de primer arranque.
 * Orquesta endpoints ya existentes: /networks/suggest, /networks,
 * /scans, /scans/{id} y /audits/snapshot. Sin endpoints nuevos,
 * sin telemetría y sin llamadas externas.
 * Depende de helpers de app.js: tr, esc, $, api, state,
 * connectWS, enterApp y startTour.
 * ============================================================ */

(function () {
  let ctx = null;

  const T = (s) => (typeof tr === 'function' ? tr(s) : s);
  const E = (v) => (typeof esc === 'function' ? esc(v) : String(v == null ? '' : v));

  function root() {
    let r = document.getElementById('onboarding-root');
    if (!r) {
      r = document.createElement('div');
      r.id = 'onboarding-root';
      r.className = 'onboarding-layer';
      document.body.appendChild(r);
    }
    return r;
  }

  function normCIDR(value) {
    const s = String(value || '').trim();
    return s && !s.includes('/') ? s + '/32' : s;
  }

  function validCIDR(value) {
    return /^(?:\d{1,3}\.){3}\d{1,3}\/(?:[0-9]|[12]\d|3[0-2])$/.test(String(value || '').trim());
  }

  function stepState(index) {
    const order = { loading: 1, choose: 1, starting: 2, scanning: 2, saving: 3, done: 4, error: 3 };
    const current = order[ctx.phase] || 1;
    if (index < current) return 'done';
    if (index === current) return 'active';
    return '';
  }

  function stepsHTML() {
    const labels = [T('Cuenta creada'), T('Red'), T('Escaneo'), T('Línea base')];
    return `<ol class="onboarding-steps">${labels.map((label, i) => `
      <li class="${stepState(i + 1)}"><span>${i + 1}</span>${E(label)}</li>`).join('')}</ol>`;
  }

  function bodyHTML() {
    if (ctx.phase === 'loading') {
      return `<div class="onboarding-center">
        <div class="spinner"></div>
        <p>${E(T('Detectando la red local…'))}</p>
      </div>`;
    }

    if (ctx.phase === 'choose') {
      const ifaceOptions = ctx.interfaces
        .filter((i) => !i.loopback)
        .map((i) => `<option value="${E(i.name)}"${i.name === ctx.iface ? ' selected' : ''}>${E(i.name)}${i.gateway ? ' · ' + E(i.gateway) : ''}</option>`)
        .join('');
      const hint = ctx.items.length
        ? T('Revisa o cambia el CIDR inicial. NetDiag escaneará esa red con el perfil común.')
        : T('No hemos encontrado una red sugerida. Puedes escribirla a mano.');
      return `
        <p class="onboarding-hint">${E(hint)}</p>
        <div class="field">
          <label for="onb-cidr">${E(T('CIDR inicial'))}</label>
          <input class="input mono" id="onb-cidr" value="${E(ctx.cidr)}" placeholder="192.168.1.0/24" autocomplete="off">
        </div>
        ${ifaceOptions ? `<div class="field">
          <label for="onb-iface">${E(T('Interfaz'))}</label>
          <select class="input" id="onb-iface"><option value="">${E(T('Detección automática'))}</option>${ifaceOptions}</select>
        </div>` : ''}
        <div class="onboarding-profile"><span class="badge ok">${E(T('Perfil común'))}</span></div>
        ${ctx.error ? `<div class="notice err">${E(ctx.error)}</div>` : ''}
        <div class="onboarding-actions">
          <button class="btn ghost" type="button" data-onb="skip">${E(T('Omitir por ahora'))}</button>
          <div class="spacer"></div>
          <button class="btn primary" type="button" data-onb="start">${E(T('Escanear y continuar'))}</button>
        </div>`;
    }

    if (ctx.phase === 'starting') {
      return `<div class="onboarding-center">
        <div class="spinner"></div>
        <p>${E(T('Buscando una red existente…'))}</p>
      </div>`;
    }

    if (ctx.phase === 'scanning') {
      return `
        <h3>${E(T('Escaneando la red'))}</h3>
        <p class="mono">${E(ctx.scanCidr || normCIDR(ctx.cidr))}</p>
        <div class="scan-progress-wrap">
          <div class="scan-progress" data-scan-progress="wizard"><i style="width:${Math.min(100, ctx.progress || 0)}%"></i></div>
          <p data-scan-label>${E(ctx.progressLabel || T('Preparando…'))}</p>
        </div>
        <p class="onboarding-hint">${E(T('El panel se abrirá con los dispositivos encontrados.'))}</p>`;
    }

    if (ctx.phase === 'saving') {
      return `<div class="onboarding-center">
        <div class="spinner"></div>
        <p>${E(T('Guardando la línea base…'))}</p>
      </div>`;
    }

    if (ctx.phase === 'error') {
      return `<div class="notice err">${E(ctx.error || T('No se pudo completar el asistente'))}</div>
        <div class="onboarding-actions">
          <button class="btn ghost" type="button" data-onb="skip">${E(T('Omitir por ahora'))}</button>
          <div class="spacer"></div>
          <button class="btn primary" type="button" data-onb="retry">${E(T('Reintentar'))}</button>
        </div>`;
    }

    return `
      <div class="onboarding-ok">${E(ctx.snapshotError ? T('Todo listo, con un aviso') : T('Todo listo'))}</div>
      <p>${E(T('El panel ya tiene hosts reales y se ha guardado una línea base de auditoría.'))}</p>
      ${ctx.snapshotError ? `<div class="notice warn">${E(ctx.snapshotError)}</div>` : ''}
      <div class="onboarding-actions">
        <button class="btn ghost" type="button" data-onb="enter">${E(T('Ver panel sin recorrido'))}</button>
        <div class="spacer"></div>
        <button class="btn primary" type="button" data-onb="tour">${E(T('Ver panel y hacer el recorrido'))}</button>
      </div>`;
  }

  function render() {
    if (!ctx) return;
    const r = root();
    r.innerHTML = `
      <div class="onboarding-card" role="dialog" aria-modal="true">
        <div class="onboarding-head">
          <div class="mark">N</div>
          <div>
            <h1>${E(T('Configuración inicial'))}</h1>
            <p>${E(T('Ya tienes la cuenta. Vamos a dejar la red lista en menos de dos minutos.'))}</p>
          </div>
        </div>
        ${stepsHTML()}
        <div class="onboarding-body">${bodyHTML()}</div>
        <div class="onboarding-foot">${E(T('Todo se guarda en este servidor. Sin telemetría.'))}</div>
      </div>`;
    bind(r);
  }

  function bind(r) {
    const cidr = $('#onb-cidr', r);
    if (cidr) cidr.addEventListener('input', (e) => { ctx.cidr = e.target.value; ctx.error = ''; });
    const iface = $('#onb-iface', r);
    if (iface) iface.addEventListener('change', (e) => { ctx.iface = e.target.value; });
    const on = (sel, fn) => {
      const el = $(sel, r);
      if (el) el.addEventListener('click', fn);
    };
    on('[data-onb="start"]', () => startScan());
    on('[data-onb="skip"]', () => finishWizard(false));
    on('[data-onb="retry"]', () => { ctx.phase = 'choose'; ctx.error = ''; render(); });
    on('[data-onb="enter"]', () => finishWizard(false));
    on('[data-onb="tour"]', () => finishWizard(true));
  }

  async function loadSuggestions() {
    try {
      const res = await api('/networks/suggest');
      ctx.items = Array.isArray(res.items) ? res.items : [];
      ctx.interfaces = Array.isArray(res.interfaces) ? res.interfaces : [];
      const pick = ctx.items.find((i) => !i.already_added) || ctx.items[0] || null;
      ctx.cidr = pick ? (pick.cidr || '') : '';
      ctx.iface = pick ? (pick.interface || '') : '';
    } catch (e) {
      ctx.items = [];
      ctx.interfaces = [];
      ctx.error = e && e.message ? e.message : T('No se pudo detectar la red local.');
    }
    ctx.phase = 'choose';
    render();
  }

  async function ensureNetwork(cidr) {
    const res = await api('/networks');
    const items = Array.isArray(res.items) ? res.items : [];
    const found = items.find((n) => normCIDR(n.cidr) === cidr && (!n.site_id || Number(n.site_id) === 1));
    if (found) return found.id;
    const created = await api('/networks', {
      method: 'POST',
      body: {
        site_id: 1,
        name: T('Red inicial'),
        cidr,
        interface: ctx.iface || '',
        enabled: true,
      },
    });
    return created && created.id;
  }

  function clearTimer() {
    if (ctx && ctx.pollTimer) {
      clearTimeout(ctx.pollTimer);
      ctx.pollTimer = null;
    }
  }

  async function pollScan() {
    if (!ctx || ctx.phase !== 'scanning' || !ctx.scanId) return;
    try {
      const scan = await api('/scans/' + ctx.scanId);
      if (scan && scan.hosts_found !== undefined) ctx.found = Number(scan.hosts_found) || 0;
      if (scan && ['done', 'error', 'canceled', 'interrupted'].includes(scan.status)) {
        completeScan(scan);
        return;
      }
    } catch (_) {
      // El WebSocket suele llegar antes; el polling es solo el respaldo.
    }
    if (ctx && ctx.phase === 'scanning') ctx.pollTimer = setTimeout(pollScan, 1200);
  }

  function completeScan(scan) {
    if (!ctx || ctx.finishing) return;
    ctx.finishing = true;
    clearTimer();
    const status = scan && scan.status;
    if (status !== 'done') {
      ctx.phase = 'error';
      ctx.error = T('El escaneo terminó con estado') + ': ' + String(status || 'error');
      render();
      return;
    }
    ctx.phase = 'saving';
    render();
    api('/audits/snapshot', {
      method: 'POST',
      body: {
        site_id: 1,
        name: T('Línea base inicial'),
        notes: T('Creada por el asistente de primer arranque'),
      },
    }).then((snap) => {
      if (!ctx) return;
      ctx.snapshotId = snap && snap.id;
      ctx.phase = 'done';
      render();
    }).catch((e) => {
      if (!ctx) return;
      ctx.snapshotError = (e && e.message) || T('No se pudo guardar la línea base.');
      ctx.phase = 'done';
      render();
    });
  }

  async function startScan() {
    const cidr = normCIDR(ctx.cidr);
    if (!validCIDR(cidr)) {
      ctx.error = T('Introduce un CIDR IPv4 válido, por ejemplo 192.168.1.0/24.');
      render();
      return;
    }
    ctx.cidr = cidr;
    ctx.phase = 'starting';
    ctx.error = '';
    ctx.finishing = false;
    render();
    try {
      const networkId = await ensureNetwork(cidr);
      const scan = await api('/scans', {
        method: 'POST',
        body: {
          site_id: 1,
          network_id: Number(networkId) || 0,
          cidr,
          profile: 'common',
        },
      });
      ctx.networkId = Number(networkId) || 0;
      ctx.scanId = scan && scan.id;
      ctx.scanCidr = (scan && scan.cidr) || cidr;
      ctx.progress = 0;
      ctx.progressLabel = T('Preparando…');
      ctx.phase = 'scanning';
      render();
      pollScan();
    } catch (e) {
      ctx.phase = 'error';
      ctx.error = (e && e.message) || T('No se pudo lanzar el escaneo.');
      render();
    }
  }

  function onProgress(data) {
    if (!ctx || ctx.phase !== 'scanning' || !data) return;
    ctx.progress = Math.min(100, Number(data.percent) || 0);
    ctx.progressLabel = (data.stage || T('Escaneando')) + ' · ' + (Number(data.found) || 0) + ' ' + T('dispositivos encontrados');
    const bar = document.querySelector('[data-scan-progress="wizard"] i');
    if (bar) bar.style.width = ctx.progress + '%';
    const label = document.querySelector('[data-scan-progress="wizard"] + p, [data-scan-label]');
    if (label) label.textContent = ctx.progressLabel;
  }

  function onFinished(data) {
    if (!ctx || ctx.phase !== 'scanning' || !data) return;
    if (ctx.scanId && Number(data.id) !== Number(ctx.scanId)) return;
    completeScan(data);
  }

  function finishWizard(withTour) {
    if (!ctx) return;
    clearTimer();
    try { localStorage.setItem('netdiag-site', '1'); } catch (_) {}
    const r = document.getElementById('onboarding-root');
    if (r) r.remove();
    ctx = null;
    Promise.resolve(enterApp({ skipTour: true })).then(() => {
      if (withTour && typeof startTour === 'function') setTimeout(startTour, 900);
    });
  }

  function stop() {
    clearTimer();
    const r = document.getElementById('onboarding-root');
    if (r) r.remove();
    ctx = null;
  }

  function startSetupWizard() {
    stop();
    ctx = {
      phase: 'loading',
      cidr: '',
      iface: '',
      items: [],
      interfaces: [],
      scanId: 0,
      networkId: 0,
      progress: 0,
      pollTimer: null,
      finishing: false,
      error: '',
    };
    if (typeof connectWS === 'function') connectWS();
    render();
    loadSuggestions();
  }

  window.NetDiagOnboarding = { startSetupWizard, onProgress, onFinished, stop };
}());
