const LANG_STORAGE_KEY = 'eop_ui_lang';

const state = {
  langMode: 'auto',
  lang: 'en',
  activeTab: 'dashboard',
  overview: null,
  templates: [],
  pendingAuths: [],
  pendingSignature: '',
  pendingPollTimer: null,
  oauthSessionId: null,
  oauthPollTimer: null,
  oauthAuthURL: '',
  oauthSummaryText: '',
  lastOAuthSession: null,
  modalSummaryText: '',
  logSource: null,
  logsPaused: false,
  logStatusKey: 'logs.disconnected',
  logLines: [],
};

function detectBrowserLanguage() {
  const candidates = [];
  if (Array.isArray(navigator.languages)) {
    candidates.push(...navigator.languages);
  }
  if (navigator.language) candidates.push(navigator.language);
  if (navigator.userLanguage) candidates.push(navigator.userLanguage);

  for (const raw of candidates) {
    const norm = String(raw || '').trim().toLowerCase();
    if (!norm) continue;
    if (norm === 'pt' || norm.startsWith('pt-')) return 'pt';
    if (norm === 'en' || norm.startsWith('en-')) return 'en';
  }
  return 'en';
}

function resolveLanguage(mode) {
  if (mode === 'en' || mode === 'pt') return mode;
  return detectBrowserLanguage();
}

function t(key, vars) {
  const dict = (window.I18N_TRANSLATIONS && window.I18N_TRANSLATIONS[state.lang]) || {};
  const fallback = (window.I18N_TRANSLATIONS && window.I18N_TRANSLATIONS.en) || {};
  let text = dict[key] ?? fallback[key] ?? key;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      text = text.replaceAll(`{${k}}`, String(v ?? ''));
    }
  }
  return text;
}

function getDateLocale() {
  return state.lang === 'pt' ? 'pt-BR' : 'en-US';
}

function applyDOMTranslations() {
  document.documentElement.lang = state.lang === 'pt' ? 'pt-BR' : 'en';
  document.title = t('page.title');

  document.querySelectorAll('[data-i18n]').forEach((el) => {
    const key = el.getAttribute('data-i18n');
    if (key) el.textContent = t(key);
  });
  document.querySelectorAll('[data-i18n-html]').forEach((el) => {
    const key = el.getAttribute('data-i18n-html');
    if (key) el.innerHTML = t(key);
  });
  document.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
    const key = el.getAttribute('data-i18n-placeholder');
    if (key) el.placeholder = t(key);
  });
  document.querySelectorAll('[data-i18n-title]').forEach((el) => {
    const key = el.getAttribute('data-i18n-title');
    if (key) el.title = t(key);
  });

  const langSelect = document.getElementById('lang-select');
  if (langSelect) {
    langSelect.value = state.langMode;
  }

  updateTabHeader(state.activeTab);

  const toggleBtn = document.getElementById('btn-logs-toggle');
  if (toggleBtn) {
    toggleBtn.textContent = t(state.logsPaused ? 'logs.resume' : 'logs.pause');
  }
  const sseBadge = document.getElementById('logs-sse-badge');
  if (sseBadge && state.logStatusKey) {
    sseBadge.textContent = t(state.logStatusKey);
  }
}

function setLanguageMode(mode, persist = true) {
  const validMode = (mode === 'en' || mode === 'pt' || mode === 'auto') ? mode : 'auto';
  state.langMode = validMode;
  state.lang = resolveLanguage(validMode);
  if (persist) {
    try {
      localStorage.setItem(LANG_STORAGE_KEY, validMode);
    } catch (_) {}
  }

  applyDOMTranslations();

  if (state.overview) {
    renderDashboard(state.overview);
    renderDebugButton(state.overview.debug_mode, state.overview.debug_known);
    state.pendingSignature = '';
    renderPendingAuths(state.pendingAuths || []);
    renderServersTable(state.overview.servers || []);
    renderConfigAccountsTable(state.overview.accounts || []);
    renderCerts(state.overview.certs || {});
    populateOAuthPorts(state.overview.servers || []);

    const portsModal = document.getElementById('ports-modal');
    if (portsModal && !portsModal.classList.contains('hidden')) {
      renderPortsModal(state.overview.docker_ports);
    }
  } else {
    renderDebugButton(false, false);
  }

  if (state.lastOAuthSession) {
    renderOAuthSession(state.lastOAuthSession);
  }

  const accModal = document.getElementById('account-modal');
  if (accModal && !accModal.classList.contains('hidden')) {
    updateAccountModalFlowUI();
  }
}

function initLanguage() {
  let saved = 'auto';
  try {
    saved = localStorage.getItem(LANG_STORAGE_KEY) || 'auto';
  } catch (_) {}
  setLanguageMode(saved, false);

  const langSelect = document.getElementById('lang-select');
  if (langSelect) {
    langSelect.addEventListener('change', (e) => {
      setLanguageMode(e.target.value, true);
    });
  }
}

function openCenteredPopup(url) {
  if (!url) return;
  const w = 560;
  const h = 740;
  const left = Math.max(0, Math.round(window.screenX + (window.outerWidth - w) / 2));
  const top = Math.max(0, Math.round(window.screenY + (window.outerHeight - h) / 2));
  const pop = window.open(
    url,
    'eop_oauth_popup',
    `width=${w},height=${h},left=${left},top=${top},scrollbars=yes,resizable=yes`
  );
  if (!pop) {
    toast(t('common.popup_blocked'), 'error');
  }
}

function getDynamicPanelCallbackURL() {
  return `${window.location.origin}/oauth/callback`;
}

function formatProtocolLabel(proto) {
  if (!proto) return '';
  if (proto.toUpperCase() === 'POP') return 'POP3';
  return proto.toUpperCase();
}

function formatSecurityLabel(srv) {
  if (srv.local_tls) return 'TLS (SSL/TLS)';
  if (srv.local_starttls) return 'STARTTLS';
  return t('dash.no_tls_local');
}

function buildAccountSummaryText(email, servers) {
  const hostExternal = window.location.hostname;
  const hostInternal = state.overview?.proxy_host || 'email-oauth2-proxy';
  const lines = [
    t('acc.summary_account', { email }),
    t('acc.summary_ext', { host: hostExternal }),
    t('acc.summary_int', { host: hostInternal }),
    t('acc.summary_user', { email }),
    t('acc.summary_pass'),
    ``,
    t('acc.summary_ports'),
  ];
  for (const s of servers) {
    const label = formatProtocolLabel(s.protocol);
    lines.push(t('acc.summary_port_line', {
      section: s.section,
      proto: label,
      port: s.local_port,
      sec: formatSecurityLabel(s),
      target: `${s.server_address}:${s.server_port}`,
    }));
  }
  return lines.join('\n');
}

function renderPortsListHTML(servers, probes) {
  if (!servers || servers.length === 0) {
    return `<div class="muted small">${t('ports.none_matching')}</div>`;
  }
  const probeBySec = new Map((probes || []).map(p => [p.section, p]));
  return `<div class="ports-list">` + servers.map(s => {
    const label = formatProtocolLabel(s.protocol);
    const sec = formatSecurityLabel(s);
    const pr = probeBySec.get(s.section);
    const statusPill = pr
      ? `<span class="badge ${pr.online ? 'badge-success' : 'badge-danger'}">${pr.online ? '● Online' : '● Offline'}</span>`
      : '';
    return `
      <div class="port-row-item">
        <div class="row">
          <span class="port-tag">${escapeHtml(s.section)}</span>
          <span class="badge badge-info">${escapeHtml(label)}</span>
          <span class="badge ${s.local_tls ? 'badge-info' : (s.local_starttls ? 'badge-warn' : 'badge-muted')}">
            ${escapeHtml(sec)}
          </span>
          ${statusPill}
        </div>
        <div class="muted small" style="font-family:var(--mono);">
          ${t('ports.port_line', { port: s.local_port, target: `${escapeHtml(s.server_address)}:${s.server_port}` })}
        </div>
      </div>
    `;
  }).join('') + `</div>`;
}

function updateTabHeader(tab) {
  const metaKeys = {
    dashboard: ['tab.dashboard.title', 'tab.dashboard.sub'],
    oauth: ['tab.oauth.title', 'tab.oauth.sub'],
    config: ['tab.config.title', 'tab.config.sub'],
    certs: ['tab.certs.title', 'tab.certs.sub'],
    logs: ['tab.logs.title', 'tab.logs.sub'],
  };
  const pair = metaKeys[tab];
  document.getElementById('page-title').textContent = pair ? t(pair[0]) : t('tab.dashboard.title');
  document.getElementById('page-subtitle').textContent = pair ? t(pair[1]) : '';
}

async function api(url, opts = {}) {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  if (res.status === 401) {
    showLoginOverlay();
    throw new Error(t('common.session_expired'));
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || t('common.http_error', { status: res.status }));
  }
  return data;
}

function toast(msg, type = 'info') {
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.textContent = msg;
  document.getElementById('toast-container').appendChild(el);
  setTimeout(() => el.remove(), 4200);
}

function confirmDialog({ title, message = '', confirmLabel, danger = true } = {}) {
  return new Promise((resolve) => {
    const overlay = document.getElementById('confirm-modal');
    const titleEl = document.getElementById('confirm-modal-title');
    const msgEl = document.getElementById('confirm-modal-message');
    const okBtn = document.getElementById('confirm-modal-ok');
    const cancelBtn = document.getElementById('confirm-modal-cancel');
    const closeBtn = document.getElementById('confirm-modal-close');

    titleEl.textContent = title || t('common.confirm_title');
    msgEl.textContent = message;
    okBtn.textContent = confirmLabel || t('common.confirm');
    okBtn.className = `btn btn-sm ${danger ? 'btn-danger' : 'btn-primary'}`;

    const cleanup = (result) => {
      overlay.classList.add('hidden');
      okBtn.removeEventListener('click', onOk);
      cancelBtn.removeEventListener('click', onCancel);
      closeBtn.removeEventListener('click', onCancel);
      overlay.removeEventListener('click', onBackdrop);
      document.removeEventListener('keydown', onKey);
      resolve(result);
    };

    const onOk = () => cleanup(true);
    const onCancel = () => cleanup(false);
    const onBackdrop = (ev) => { if (ev.target === overlay) cleanup(false); };
    const onKey = (ev) => { if (ev.key === 'Escape') cleanup(false); };

    okBtn.addEventListener('click', onOk);
    cancelBtn.addEventListener('click', onCancel);
    closeBtn.addEventListener('click', onCancel);
    overlay.addEventListener('click', onBackdrop);
    document.addEventListener('keydown', onKey);

    overlay.classList.remove('hidden');
    okBtn.focus();
  });
}

function escapeHtml(str) {
  return String(str ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function formatUnix(ts) {
  if (!ts) return '—';
  const d = new Date(ts * 1000);
  return d.toLocaleString(getDateLocale(), { dateStyle: 'short', timeStyle: 'short' });
}

function showReloadHint(reloadRes) {
  const banner = document.getElementById('reload-banner');
  const txt = document.getElementById('reload-banner-text');
  const btn = document.getElementById('btn-banner-restart');
  if (reloadRes && reloadRes.reloaded) {
    toast(reloadRes.message, 'success');
    banner.classList.add('hidden');
    return;
  }
  txt.textContent = (reloadRes && reloadRes.message)
    ? reloadRes.message
    : t('banner.default_reload');
  const canRestart = !!(reloadRes?.supported || state.overview?.restart_available);
  btn.classList.toggle('hidden', !canRestart);
  banner.classList.remove('hidden');
}

async function restartProxyNow() {
  const sideBtn = document.getElementById('btn-reload-proxy');
  const banBtn = document.getElementById('btn-banner-restart');
  sideBtn.disabled = true;
  banBtn.disabled = true;
  try {
    const res = await api('/api/proxy/reload', { method: 'POST' });
    showReloadHint(res);
    if (res.reloaded) {
      setTimeout(loadOverview, 2000);
    }
  } catch (err) {
    toast(err.message, 'error');
  } finally {
    sideBtn.disabled = false;
    banBtn.disabled = false;
  }
}
window.restartProxyNow = restartProxyNow;

function hideReloadBanner() {
  document.getElementById('reload-banner').classList.add('hidden');
}

function switchTab(tab) {
  state.activeTab = tab;
  document.querySelectorAll('.nav-item').forEach(b => b.classList.toggle('active', b.dataset.tab === tab));
  document.querySelectorAll('.tab-panel').forEach(p => p.classList.toggle('active', p.id === `tab-${tab}`));
  updateTabHeader(tab);

  if (tab === 'logs') {
    connectLogStream();
  } else {
    disconnectLogStream();
  }
  if (tab === 'config') {
    loadRawConfig();
  }
}

window.switchTab = switchTab;
window.hideReloadBanner = hideReloadBanner;
window.closeModal = (id) => document.getElementById(id).classList.add('hidden');

// ==================== AUTH ====================
function showLoginOverlay() {
  document.getElementById('login-overlay').classList.remove('hidden');
}

async function checkAuth() {
  const st = await fetch('/api/auth/status').then(r => r.json());
  if (st.enabled) {
    document.getElementById('btn-logout').classList.remove('hidden');
  }
  if (st.enabled && !st.authenticated) {
    showLoginOverlay();
    return false;
  }
  document.getElementById('login-overlay').classList.add('hidden');
  return true;
}

document.getElementById('login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const errEl = document.getElementById('login-error');
  errEl.classList.add('hidden');
  try {
    await api('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        username: document.getElementById('login-user').value,
        password: document.getElementById('login-pass').value,
      }),
    });
    document.getElementById('login-overlay').classList.add('hidden');
    await bootstrap();
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('hidden');
  }
});

document.getElementById('btn-logout').addEventListener('click', async () => {
  await fetch('/api/auth/logout', { method: 'POST' });
  showLoginOverlay();
});

// ==================== OVERVIEW / DASHBOARD ====================
async function loadOverview() {
  const data = await api('/api/overview');
  state.overview = data;
  renderDashboard(data);
  renderDebugButton(data.debug_mode, data.debug_known);
  renderPendingAuths(data.pending_auths || []);
  syncPortModeSelects(data.docker_ports);
  renderServersTable(data.servers || []);
  renderConfigAccountsTable(data.accounts || []);
  renderGlobals(data.globals || {});
  renderCerts(data.certs || {});
  populateOAuthPorts(data.servers || []);
}

function renderDebugButton(debugMode, debugKnown) {
  const btn = document.getElementById('btn-toggle-debug');
  if (!btn) return;
  if (!debugKnown) {
    btn.textContent = t('sidebar.debug_unknown');
  } else {
    btn.textContent = debugMode ? t('sidebar.debug_on') : t('sidebar.debug_off');
  }
}

// ==================== EXTERNAL PENDING OAUTH REQUESTS ====================
function renderPendingAuths(list) {
  state.pendingAuths = list || [];
  const sig = state.pendingAuths.map(p => `${p.id}:${p.auth_url}:${p.device_code || ''}`).join('|');

  const navBadge = document.getElementById('nav-pending-badge');
  const card = document.getElementById('pending-auth-card');
  const countBadge = document.getElementById('pending-auth-count');
  const container = document.getElementById('pending-auth-list');

  if (state.pendingAuths.length === 0) {
    navBadge.classList.add('hidden');
    card.classList.add('hidden');
    state.pendingSignature = '';
    container.innerHTML = '';
    return;
  }

  navBadge.textContent = String(state.pendingAuths.length);
  navBadge.classList.remove('hidden');
  countBadge.textContent = String(state.pendingAuths.length);
  card.classList.remove('hidden');

  if (sig === state.pendingSignature) return;
  state.pendingSignature = sig;

  container.innerHTML = state.pendingAuths.map((p, idx) => `
    <div class="subcard">
      <div class="card-header" style="margin-bottom:0;">
        <div>
          <strong>${escapeHtml(p.email)}</strong>
          <span class="muted small" style="margin-left:8px;">${escapeHtml(t('pending.detected_at', { time: new Date(p.detected_at).toLocaleTimeString(getDateLocale()) }))}</span>
        </div>
        <button type="button" class="btn btn-ghost btn-xs" onclick="dismissPendingByIndex(${idx})">${escapeHtml(t('pending.dismiss'))}</button>
      </div>
      <div class="row">
        <button type="button" class="btn btn-primary btn-sm" onclick="openPendingPopupByIndex(${idx})">${escapeHtml(t('pending.open_login'))}</button>
        <button type="button" class="btn btn-secondary btn-sm" onclick="copyPendingURLByIndex(${idx})">${escapeHtml(t('pending.copy_url'))}</button>
      </div>
      ${p.device_code
        ? `<div class="device-code-row">
            <code class="device-code">${escapeHtml(p.device_code)}</code>
            <button type="button" class="btn btn-secondary btn-xs" onclick="copyPendingCodeByIndex(${idx})">${escapeHtml(t('pending.copy_code'))}</button>
          </div>`
        : `<form class="row" style="flex-wrap:nowrap;" onsubmit="submitPendingCallbackForm(event, ${idx})">
            <input type="text" id="pending-cb-input-${idx}" placeholder="${escapeHtml(t('pending.cb_placeholder'))}" onpaste="handlePendingPaste(event, ${idx})" required />
            <button type="submit" class="btn btn-success btn-sm" style="white-space:nowrap;">${escapeHtml(t('pending.send_cb'))}</button>
          </form>`}
    </div>
  `).join('');
}

window.openPendingPopupByIndex = (idx) => {
  const p = state.pendingAuths[idx];
  if (p) openCenteredPopup(p.auth_url);
};

window.copyPendingURLByIndex = (idx) => {
  const p = state.pendingAuths[idx];
  if (p) navigator.clipboard.writeText(p.auth_url).then(() => toast(t('common.link_copied'), 'success'));
};

window.copyPendingCodeByIndex = (idx) => {
  const p = state.pendingAuths[idx];
  if (p && p.device_code) navigator.clipboard.writeText(p.device_code).then(() => toast(t('common.code_copied'), 'success'));
};

async function submitPendingCallback(idx, urlVal) {
  const p = state.pendingAuths[idx];
  if (!p || !urlVal) return;
  try {
    const res = await api(`/api/oauth/pending/${encodeURIComponent(p.id)}/callback`, {
      method: 'POST',
      body: JSON.stringify({ url: urlVal }),
    });
    toast(t('pending.cb_sent', { email: p.email }), 'success');
    renderPendingAuths(res.pending || []);
    setTimeout(loadOverview, 1200);
  } catch (err) {
    toast(err.message, 'error');
  }
}

window.submitPendingCallbackForm = async (e, idx) => {
  e.preventDefault();
  const inp = document.getElementById(`pending-cb-input-${idx}`);
  await submitPendingCallback(idx, inp ? inp.value.trim() : '');
};

window.handlePendingPaste = (e, idx) => {
  const pasted = (e.clipboardData || window.clipboardData)?.getData('text')?.trim() || '';
  if (pasted && (pasted.includes('code=') || pasted.includes('error='))) {
    setTimeout(() => submitPendingCallback(idx, pasted), 60);
  }
};

window.dismissPendingByIndex = async (idx) => {
  const p = state.pendingAuths[idx];
  if (!p) return;
  try {
    const res = await api(`/api/oauth/pending/${encodeURIComponent(p.id)}`, { method: 'DELETE' });
    renderPendingAuths(res.pending || []);
  } catch (err) {
    toast(err.message, 'error');
  }
};

function startPendingPolling() {
  if (state.pendingPollTimer) return;
  state.pendingPollTimer = setInterval(async () => {
    if (document.hidden || !document.getElementById('login-overlay').classList.contains('hidden')) return;
    try {
      const res = await fetch('/api/oauth/pending');
      if (!res.ok) return;
      const data = await res.json();
      const prevCount = state.pendingAuths.length;
      const list = data.pending || [];
      renderPendingAuths(list);
      if (list.length > prevCount) {
        toast(t('pending.new_toast', { email: list[0].email }), 'info');
      }
    } catch (_) {}
  }, 4000);
}

function renderDashboard(data) {
  document.getElementById('sidebar-proxy-host').textContent = `${t('sidebar.proxy_prefix')}: ${data.proxy_host}`;
  document.getElementById('proxy-host-label').textContent = t('dash.internal_host', { host: data.proxy_host });

  const probes = data.probes || [];
  const onlineCount = probes.filter(p => p.online).length;
  document.getElementById('kpi-ports').textContent = `${onlineCount} / ${probes.length}`;
  document.getElementById('kpi-ports-sub').textContent = onlineCount === probes.length && probes.length > 0
    ? t('dash.kpi_ports_all_ok') : (probes.length === 0 ? t('dash.kpi_ports_none') : t('dash.kpi_ports_partial'));

  const accounts = data.accounts || [];
  const realAccounts = accounts.filter(a => !a.is_catch_all);
  const authAccounts = realAccounts.filter(a => a.token_status === 'authenticated').length;
  document.getElementById('kpi-accounts').textContent = `${authAccounts} / ${realAccounts.length}`;
  document.getElementById('kpi-accounts-sub').textContent = t('dash.kpi_accounts_catchall', {
    count: accounts.filter(a => a.is_catch_all).length,
  });

  const tlsEnabledServers = (data.servers || []).filter(s => s.local_tls || s.local_starttls).length;
  document.getElementById('kpi-tls').textContent = data.certs?.has_server_cert ? t('dash.kpi_tls_active') : t('dash.kpi_tls_missing');
  document.getElementById('kpi-tls-sub').textContent = t('dash.kpi_tls_sub', { count: tlsEnabledServers });

  document.getElementById('kpi-catchall').textContent = data.globals?.allow_catch_all_accounts ? t('dash.kpi_catchall_on') : t('dash.kpi_catchall_off');
  document.getElementById('kpi-catchall-sub').textContent = data.globals?.allow_catch_all_accounts
    ? t('dash.kpi_catchall_sub_on') : t('dash.kpi_catchall_sub_off');

  const dp = data.docker_ports || {};
  const dpStatusByPort = new Map((dp.statuses || []).map(st => [st.port, st]));
  const unpubCount = (dp.statuses || []).filter(st => !st.protected && !st.published_on_proxy).length;
  const pendingBadge = document.getElementById('docker-ports-pending-badge');
  if (dp.docker_available && unpubCount > 0) {
    pendingBadge.textContent = t('dash.ports_internal_count', { count: unpubCount });
    pendingBadge.classList.remove('hidden');
  } else {
    pendingBadge.classList.add('hidden');
  }
  document.getElementById('pid-shared-warn').classList.toggle('hidden', !dp.self_pid_shared);

  // Ports grid
  const portsGrid = document.getElementById('ports-grid');
  if (probes.length === 0) {
    portsGrid.innerHTML = `<div class="empty-state">${t('dash.no_servers_empty')}</div>`;
  } else {
    portsGrid.innerHTML = probes.map(p => {
      const dst = dpStatusByPort.get(p.local_port);
      let dockerBadge = '';
      if (dp.docker_available && dst) {
        if (dst.published_on_proxy) {
          dockerBadge = `<span class="badge badge-success">${escapeHtml(t('dash.docker_open'))}</span>`;
        } else if (dst.status === 'conflict_docker' || dst.status === 'conflict_host') {
          dockerBadge = `<span class="badge badge-danger" title="${escapeHtml(dst.message)}">${escapeHtml(t('dash.docker_conflict'))}</span>`;
        } else {
          dockerBadge = `<span class="badge badge-warn" title="${escapeHtml(t('dash.docker_internal_title'))}">${escapeHtml(t('dash.docker_internal'))}</span>`;
        }
      }
      return `
        <div class="port-tile">
          <div class="port-tile-top">
            <span class="port-title">${escapeHtml(p.section)}</span>
            <span class="badge ${p.online ? 'badge-success' : 'badge-danger'}">
              ${p.online ? `● Online (${p.latency_ms}ms)` : '● Offline'}
            </span>
          </div>
          <div class="port-target">→ ${escapeHtml(p.server_address)}:${p.server_port}</div>
          <div class="row">
            <span class="badge ${p.local_tls ? 'badge-info' : (p.local_starttls ? 'badge-warn' : 'badge-muted')}">
              ${p.local_tls ? 'TLS' : (p.local_starttls ? 'STARTTLS' : escapeHtml(t('dash.no_tls')))}
            </span>
            ${dockerBadge}
          </div>
          ${p.error ? `<div class="small muted" title="${escapeHtml(p.error)}">${escapeHtml(p.error.slice(0, 75))}</div>` : ''}
        </div>
      `;
    }).join('');
  }

  // Accounts table
  const tbody = document.getElementById('accounts-tbody');
  if (accounts.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" class="muted" style="text-align:center;padding:24px;">${escapeHtml(t('dash.accounts_empty'))}</td></tr>`;
  } else {
    tbody.innerHTML = accounts.map((a, idx) => {
      let statusBadge = `<span class="badge badge-warn">${escapeHtml(t('dash.status_pending'))}</span>`;
      if (a.token_status === 'authenticated') statusBadge = `<span class="badge badge-success">${escapeHtml(t('dash.status_authenticated'))}</span>`;
      else if (a.token_status === 'expired_access') statusBadge = `<span class="badge badge-danger">${escapeHtml(t('dash.status_expired'))}</span>`;
      else if (a.token_status === 'catch_all') statusBadge = `<span class="badge badge-info">${escapeHtml(t('dash.status_catch_all'))}</span>`;

      const isAuth = a.token_status === 'authenticated';
      const hasAnyToken = !!(a.has_refresh_token || a.has_access_token);

      const statusCell = `<div class="cell-stack">
        ${statusBadge}
        ${!a.is_catch_all && hasAnyToken ? `<button class="btn btn-secondary btn-xs" onclick="clearTokenByIndex(${idx})">${escapeHtml(t('dash.btn_clear'))}</button>` : ''}
      </div>`;

      const refreshCell = a.is_catch_all
        ? `<span class="muted">—</span>`
        : (a.has_refresh_token
          ? `<span class="badge badge-success">${escapeHtml(t('dash.refresh_saved'))}</span>`
          : `<div class="cell-stack">
              <span class="badge badge-muted">${escapeHtml(t('dash.refresh_none'))}</span>
              <button class="btn btn-primary btn-xs" onclick="prefillOAuthByIndex(${idx})">${escapeHtml(t('dash.btn_auth'))}</button>
            </div>`);

      const timeInfo = a.is_catch_all
        ? `<span class="muted small">Client: ${escapeHtml((a.client_id || '').slice(0, 22))}…</span>`
        : `<div class="small">${escapeHtml(t('dash.expires_prefix', { val: formatUnix(a.access_token_expiry) }))}</div><div class="muted small">${escapeHtml(t('dash.last_used_prefix', { val: formatUnix(a.last_activity) }))}</div>`;

      const actions = `<button class="btn btn-danger btn-xs" onclick="deleteAccountByIndex(${idx}, ${!a.is_catch_all})">${escapeHtml(t('dash.btn_remove'))}</button>`;

      const portText = (!a.is_catch_all && (a.matched_servers || []).length > 0)
        ? `<span class="muted small" style="font-family:var(--mono);">${a.matched_servers.map(s => escapeHtml(s.section)).join(' · ')}</span>`
        : '';

      const accountBottomRow = (!a.is_catch_all && (isAuth || portText))
        ? `<div class="row">
            ${isAuth ? `<button class="btn btn-secondary btn-xs" onclick="openAccountInfoByIndex(${idx})">${escapeHtml(t('dash.btn_ports'))}</button>` : ''}
            ${portText}
          </div>`
        : '';

      return `
        <tr>
          <td>
            <div class="cell-stack">
              <div class="row">
                <strong>${escapeHtml(a.account)}</strong>
                ${a.from_cache_only ? `<span class="badge badge-muted">${escapeHtml(t('dash.via_catchall'))}</span>` : ''}
              </div>
              ${accountBottomRow}
            </div>
          </td>
          <td>${statusCell}</td>
          <td>${refreshCell}</td>
          <td>${timeInfo}</td>
          <td class="right"><div class="row right">${actions}</div></td>
        </tr>
      `;
    }).join('');
  }
}

window.openAccountInfoByIndex = (idx) => {
  const acc = state.overview?.accounts?.[idx];
  if (!acc) return;

  const servers = (acc.matched_servers && acc.matched_servers.length > 0)
    ? acc.matched_servers
    : (state.overview?.servers || []);
  const probes = state.overview?.probes || [];
  const hostExternal = window.location.hostname;
  const hostInternal = state.overview?.proxy_host || 'email-oauth2-proxy';

  state.modalSummaryText = buildAccountSummaryText(acc.account, servers);

  document.getElementById('acc-modal-title').textContent = acc.account;
  const statusStr = acc.token_status === 'authenticated' ? t('dash.status_authenticated') : acc.token_status;
  document.getElementById('acc-modal-sub').textContent =
    `Status: ${statusStr} · ${t('dash.expires_prefix', { val: formatUnix(acc.access_token_expiry) })} · ${t('dash.last_used_prefix', { val: formatUnix(acc.last_activity) })}`;

  const serverListStr = servers.map(s => escapeHtml(s.section)).join(', ') || escapeHtml(t('acc.none_lower'));
  document.getElementById('acc-modal-body').innerHTML = `
    <div class="subcard">
      <div class="subcard-title">${escapeHtml(t('acc.info_conn_title'))}</div>
      <div class="small"><strong>${escapeHtml(t('acc.info_ext_server'))}</strong> <code>${escapeHtml(hostExternal)}</code></div>
      <div class="small"><strong>${escapeHtml(t('acc.info_int_server'))}</strong> <code>${escapeHtml(hostInternal)}</code></div>
      <div class="small"><strong>${escapeHtml(t('acc.info_user'))}</strong> <code>${escapeHtml(acc.account)}</code></div>
      <div class="small"><strong>${escapeHtml(t('acc.info_pass'))}</strong> <span class="muted">${escapeHtml(t('acc.info_pass_hint'))}</span></div>
    </div>
    <div class="subcard">
      <div class="subcard-title">${t('acc.info_ports_title', { list: serverListStr })}</div>
      ${renderPortsListHTML(servers, probes)}
    </div>
  `;

  document.getElementById('account-info-modal').classList.remove('hidden');
};

document.getElementById('btn-copy-acc-summary').addEventListener('click', () => {
  if (!state.modalSummaryText) return;
  navigator.clipboard.writeText(state.modalSummaryText).then(() => toast(t('acc.summary_copied'), 'success'));
});

window.prefillOAuthByIndex = (idx) => {
  const acc = state.overview?.accounts?.[idx];
  if (!acc) return;
  document.getElementById('oauth-email').value = acc.account;
  switchTab('oauth');
  document.getElementById('oauth-password').focus();
};

window.clearTokenByIndex = async (idx) => {
  const acc = state.overview?.accounts?.[idx];
  if (!acc) return;
  const account = acc.account;
  const ok = await confirmDialog({
    title: t('common.clear_tokens_title'),
    message: t('common.clear_tokens_msg', { account }),
    confirmLabel: t('dash.btn_clear'),
  });
  if (!ok) return;
  try {
    const res = await api(`/api/tokens/${encodeURIComponent(account)}`, { method: 'DELETE' });
    toast(t('common.tokens_removed', { account }), 'success');
    showReloadHint(res.reload);
    await loadOverview();
  } catch (err) {
    toast(err.message, 'error');
  }
};

window.deleteAccountByIndex = async (idx, clearToken) => {
  const acc = state.overview?.accounts?.[idx];
  if (!acc) return;
  const account = acc.account;
  const ok = await confirmDialog({
    title: t('common.delete_acc_title'),
    message: clearToken
      ? t('common.delete_acc_msg_with_token', { account })
      : t('common.delete_acc_msg_no_token', { account }),
    confirmLabel: t('dash.btn_remove'),
  });
  if (!ok) return;
  try {
    const res = await api(`/api/accounts/${encodeURIComponent(account)}?clear_token=${clearToken}`, { method: 'DELETE' });
    toast(t('common.account_removed', { account }), 'success');
    showReloadHint(res.reload);
    await loadOverview();
  } catch (err) {
    toast(err.message, 'error');
  }
};

// ==================== OAUTH WIZARD ====================
function populateOAuthPorts(servers) {
  const sel = document.getElementById('oauth-port');
  const current = sel.value;
  const imaps = servers.filter(s => s.protocol === 'IMAP');
  sel.innerHTML = `<option value="">${escapeHtml(t('oauth.port_auto'))}</option>` +
    imaps.map(s => `<option value="${s.local_port}">${escapeHtml(t('oauth.port_option', { port: s.local_port, host: s.server_address, tls: s.local_tls ? 'TLS' : t('dash.no_tls') }))}</option>`).join('');
  sel.value = current;
}

document.getElementById('oauth-start-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const email = document.getElementById('oauth-email').value.trim();
  const password = document.getElementById('oauth-password').value;
  const portVal = document.getElementById('oauth-port').value;

  stopOAuthPolling();
  document.getElementById('btn-oauth-start').disabled = true;
  try {
    const sess = await api('/api/oauth/start', {
      method: 'POST',
      body: JSON.stringify({
        email,
        password,
        port: portVal ? parseInt(portVal, 10) : 0,
      }),
    });
    state.oauthSessionId = sess.id;
    document.getElementById('oauth-callback-input').value = '';
    renderOAuthSession(sess);
    startOAuthPolling(sess.id);
  } catch (err) {
    toast(err.message, 'error');
    document.getElementById('btn-oauth-start').disabled = false;
  }
});

document.getElementById('btn-oauth-cancel').addEventListener('click', async () => {
  if (!state.oauthSessionId) return;
  try {
    await api(`/api/oauth/session/${state.oauthSessionId}/cancel`, { method: 'POST' });
  } catch (_) {}
  stopOAuthPolling();
  document.getElementById('btn-oauth-start').disabled = false;
  document.getElementById('btn-oauth-cancel').classList.add('hidden');
  toast(t('oauth.cancelled_toast'), 'info');
});

async function submitOAuthCallbackURL(pasted) {
  if (!state.oauthSessionId || !pasted) return;
  const btn = document.getElementById('btn-oauth-submit-cb');
  if (btn.disabled) return;
  btn.disabled = true;
  try {
    const res = await api(`/api/oauth/session/${state.oauthSessionId}/callback`, {
      method: 'POST',
      body: JSON.stringify({ url: pasted }),
    });
    toast(t('oauth.cb_sent_waiting'), 'info');
    if (res.session) renderOAuthSession(res.session);
  } catch (err) {
    toast(err.message, 'error');
  } finally {
    btn.disabled = false;
  }
}

document.getElementById('oauth-callback-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  await submitOAuthCallbackURL(document.getElementById('oauth-callback-input').value.trim());
});

document.getElementById('oauth-callback-input').addEventListener('paste', (e) => {
  const pasted = (e.clipboardData || window.clipboardData)?.getData('text')?.trim() || '';
  if (pasted && (pasted.includes('code=') || pasted.includes('error='))) {
    setTimeout(() => {
      submitOAuthCallbackURL(pasted);
    }, 60);
  }
});

document.getElementById('oauth-open-popup').addEventListener('click', () => {
  openCenteredPopup(state.oauthAuthURL);
});

window.addEventListener('message', async (ev) => {
  if (ev.origin !== window.location.origin) return;
  const data = ev.data;
  if (!data || data.type !== 'eop_oauth_callback') return;
  if (data.ok) {
    toast(t('oauth.auto_cb_received'), 'success');
    if (state.oauthSessionId) {
      try {
        const sess = await api(`/api/oauth/session/${state.oauthSessionId}`);
        renderOAuthSession(sess);
      } catch (_) {}
    }
  } else if (data.message) {
    toast(data.message, 'error');
  }
});

document.getElementById('btn-copy-auth-url').addEventListener('click', () => {
  if (!state.oauthAuthURL) return;
  navigator.clipboard.writeText(state.oauthAuthURL).then(() => toast(t('common.link_copied'), 'success'));
});

document.getElementById('btn-copy-device-code').addEventListener('click', () => {
  const code = document.getElementById('oauth-device-code').textContent;
  navigator.clipboard.writeText(code).then(() => toast(t('common.code_copied'), 'success'));
});

document.getElementById('btn-copy-oauth-summary').addEventListener('click', () => {
  if (!state.oauthSummaryText) return;
  navigator.clipboard.writeText(state.oauthSummaryText).then(() => toast(t('acc.summary_copied'), 'success'));
});

function startOAuthPolling(id) {
  state.oauthPollTimer = setInterval(async () => {
    try {
      const sess = await api(`/api/oauth/session/${id}`);
      renderOAuthSession(sess);
      if (['success', 'error', 'cancelled'].includes(sess.status)) {
        stopOAuthPolling();
        document.getElementById('btn-oauth-start').disabled = false;
        if (sess.status === 'success') {
          toast(t('oauth.auth_success_toast', { email: sess.email }), 'success');
          loadOverview();
        }
      }
    } catch (_) {
      stopOAuthPolling();
      document.getElementById('btn-oauth-start').disabled = false;
    }
  }, 1000);
}

function stopOAuthPolling() {
  if (state.oauthPollTimer) {
    clearInterval(state.oauthPollTimer);
    state.oauthPollTimer = null;
  }
}

function renderOAuthSession(sess) {
  state.lastOAuthSession = sess;
  document.getElementById('oauth-idle').classList.add('hidden');
  document.getElementById('oauth-active').classList.remove('hidden');

  const badge = document.getElementById('oauth-status-badge');
  const msgEl = document.getElementById('oauth-message');
  const urlBox = document.getElementById('oauth-url-box');
  const devBox = document.getElementById('oauth-device-box');
  const cbBox = document.getElementById('oauth-callback-box');
  const okBox = document.getElementById('oauth-success-box');
  const cancelBtn = document.getElementById('btn-oauth-cancel');

  const badgeMap = {
    connecting: ['badge-info', t('oauth.status_connecting')],
    awaiting_user: ['badge-warn', t('oauth.status_awaiting')],
    exchanging: ['badge-info', t('oauth.status_exchanging')],
    success: ['badge-success', t('oauth.status_success')],
    error: ['badge-danger', t('oauth.status_error')],
    cancelled: ['badge-muted', t('oauth.status_cancelled')],
  };
  const [cls, label] = badgeMap[sess.status] || ['badge-muted', sess.status];
  badge.className = `badge ${cls}`;
  badge.textContent = label;

  msgEl.className = `alert ${sess.status === 'success' ? 'alert-success' : (sess.status === 'error' ? 'alert-error' : 'alert-info')}`;
  msgEl.textContent = sess.message || '';

  const inProgress = ['connecting', 'awaiting_user', 'exchanging'].includes(sess.status);
  cancelBtn.classList.toggle('hidden', !inProgress);

  const panelCb = getDynamicPanelCallbackURL();
  const isAutoCallback = !!(sess.redirect_uri && sess.redirect_uri.replace(/\/+$/, '') === panelCb.replace(/\/+$/, ''));

  if (sess.auth_url && inProgress) {
    state.oauthAuthURL = sess.auth_url;
    urlBox.classList.remove('hidden');
    document.getElementById('oauth-url-help').innerHTML = isAutoCallback
      ? t('oauth.help_auto_cb', { url: escapeHtml(panelCb) })
      : t('oauth.help_manual_cb', { redirect: escapeHtml(sess.redirect_uri || 'localhost') });
  } else {
    urlBox.classList.add('hidden');
  }

  if (sess.device_code && inProgress) {
    devBox.classList.remove('hidden');
    document.getElementById('oauth-device-code').textContent = sess.device_code;
    cbBox.classList.add('hidden');
  } else {
    devBox.classList.add('hidden');
    cbBox.classList.toggle('hidden', !(sess.auth_url && inProgress));
    document.getElementById('oauth-cb-title').textContent = isAutoCallback
      ? t('oauth.step_b_title_auto')
      : t('oauth.step_b_title_manual');
  }

  if (sess.status === 'success') {
    okBox.classList.remove('hidden');
    const hostIP = window.location.hostname;
    const proxyHost = state.overview?.proxy_host || 'email-oauth2-proxy';
    const servers = (sess.matched_servers && sess.matched_servers.length > 0)
      ? sess.matched_servers
      : [{ section: `IMAP-${sess.port}`, protocol: 'IMAP', local_port: sess.port, local_tls: sess.use_tls, server_address: '', server_port: 0 }];
    state.oauthSummaryText = buildAccountSummaryText(sess.email, servers);

    const serverListStr = servers.map(s => escapeHtml(s.section)).join(', ');
    document.getElementById('oauth-client-hints').innerHTML = `
      <div><strong>${escapeHtml(t('acc.info_ext_server'))}</strong> <code>${escapeHtml(hostIP)}</code></div>
      <div><strong>${escapeHtml(t('acc.info_int_server'))}</strong> <code>${escapeHtml(proxyHost)}</code></div>
      <div><strong>${escapeHtml(t('acc.info_user'))}</strong> <code>${escapeHtml(sess.email)}</code> · <strong>${escapeHtml(t('acc.info_pass'))}</strong> ${escapeHtml(t('acc.info_pass_hint_oauth'))}</div>
      <div style="margin-top:4px;"><strong>${t('acc.info_ports_title', { list: serverListStr })}</strong></div>
      ${renderPortsListHTML(servers, state.overview?.probes || [])}
      ${sess.imap_response ? `<div class="muted" style="margin-top:4px;">${escapeHtml(t('oauth.imap_response'))} <code>${escapeHtml(sess.imap_response)}</code></div>` : ''}
    `;
  } else {
    okBox.classList.add('hidden');
  }
}

// ==================== TEMPLATES & CONFIG ====================
async function loadTemplates() {
  const res = await api('/api/templates');
  state.templates = res.templates || [];
  const provSel = document.getElementById('tpl-provider');
  provSel.innerHTML = state.templates.map(t => `<option value="${t.id}">${escapeHtml(t.name)}</option>`).join('');
  updateTemplatePresets();
}

function updateTemplatePresets() {
  const provId = document.getElementById('tpl-provider').value;
  const tpl = state.templates.find(t => t.id === provId);
  if (!tpl) return;

  const presetSel = document.getElementById('tpl-preset');
  presetSel.innerHTML = tpl.presets.map(p => `<option value="${p.id}">${escapeHtml(p.label)}</option>`).join('');
  document.getElementById('tpl-account').value = tpl.default_domains[0] || '';
  updatePresetDetails();
}

function updatePresetDetails() {
  const provId = document.getElementById('tpl-provider').value;
  const presetId = document.getElementById('tpl-preset').value;
  const tpl = state.templates.find(t => t.id === provId);
  const preset = tpl?.presets.find(p => p.id === presetId);
  if (!preset) return;

  document.getElementById('tpl-preset-note').textContent = preset.source_note || '';
  const isCustom = !preset.client_id;
  document.getElementById('tpl-custom-creds').classList.toggle('hidden', !isCustom);
  document.getElementById('tpl-client-id').value = preset.client_id || '';
  document.getElementById('tpl-client-secret').value = preset.client_secret || '';
  const dynamicCb = getDynamicPanelCallbackURL();
  document.getElementById('tpl-redirect-uri').placeholder = dynamicCb;
  document.getElementById('tpl-redirect-uri').value = (isCustom && preset.oauth2_flow !== 'device')
    ? dynamicCb
    : (preset.redirect_uri || '');
}

document.getElementById('btn-use-panel-callback').addEventListener('click', () => {
  const url = getDynamicPanelCallbackURL();
  document.getElementById('tpl-redirect-uri').value = url;
  toast(t('cfg.redirect_set_toast', { url }), 'info');
});

document.getElementById('tpl-provider').addEventListener('change', updateTemplatePresets);
document.getElementById('tpl-preset').addEventListener('change', updatePresetDetails);
document.getElementById('tpl-create-servers').addEventListener('change', (e) => {
  document.getElementById('tpl-port-mode-wrap').classList.toggle('hidden', !e.target.checked);
});

document.getElementById('template-apply-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const portMode = document.getElementById('tpl-create-servers').checked
    ? document.getElementById('tpl-port-mode').value
    : 'none';
  try {
    const res = await api('/api/templates/apply', {
      method: 'POST',
      body: JSON.stringify({
        provider_id: document.getElementById('tpl-provider').value,
        preset_id: document.getElementById('tpl-preset').value,
        account_target: document.getElementById('tpl-account').value.trim(),
        client_id: document.getElementById('tpl-client-id').value.trim(),
        client_secret: document.getElementById('tpl-client-secret').value.trim(),
        redirect_uri: document.getElementById('tpl-redirect-uri').value.trim(),
        create_servers: document.getElementById('tpl-create-servers').checked,
        enable_tls: document.getElementById('tpl-enable-tls').checked,
        enable_catch_all: document.getElementById('tpl-catch-all').checked,
        port_mode: portMode,
      }),
    });
    toast(t('cfg.template_applied', { account: res.account }), 'success');
    showReloadHint(res.reload);
    await loadOverview();
    await loadRawConfig();
  } catch (err) {
    toast(err.message, 'error');
  }
});

// Servers Table & Modal
function renderServersTable(servers) {
  const tbody = document.getElementById('servers-tbody');
  if (servers.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" class="muted" style="text-align:center;padding:20px;">${escapeHtml(t('cfg.servers_empty'))}</td></tr>`;
    return;
  }
  tbody.innerHTML = servers.map((s, idx) => `
    <tr>
      <td><code>[${escapeHtml(s.section)}]</code></td>
      <td><code>${escapeHtml(s.local_address)}:${s.local_port}</code></td>
      <td><code>${escapeHtml(s.server_address)}:${s.server_port}</code> ${s.server_starttls ? '<span class="badge badge-muted">STARTTLS</span>' : '<span class="badge badge-muted">SSL/TLS</span>'}</td>
      <td>${s.local_tls ? '<span class="badge badge-info">TLS</span>' : (s.local_starttls ? '<span class="badge badge-warn">STARTTLS</span>' : `<span class="badge badge-muted">${escapeHtml(t('dash.no_tls'))}</span>`)}</td>
      <td class="right">
        <button class="btn btn-secondary btn-xs" onclick="editServerByIndex(${idx})">${escapeHtml(t('cfg.btn_edit'))}</button>
        <button class="btn btn-danger btn-xs" onclick="deleteServerByIndex(${idx})">${escapeHtml(t('dash.btn_remove'))}</button>
      </td>
    </tr>
  `).join('');
}

document.getElementById('btn-add-server').addEventListener('click', () => {
  document.getElementById('server-modal-title').textContent = t('srv.add_title');
  document.getElementById('srv-orig-section').value = '';
  document.getElementById('srv-protocol').value = 'IMAP';
  document.getElementById('srv-local-port').value = '1993';
  document.getElementById('srv-remote-host').value = '';
  document.getElementById('srv-remote-port').value = '993';
  document.getElementById('srv-local-addr').value = '0.0.0.0';
  document.getElementById('srv-server-starttls').checked = false;
  document.getElementById('srv-local-tls').checked = !!state.overview?.certs?.has_server_cert;
  document.getElementById('srv-local-starttls').checked = false;
  document.getElementById('srv-port-mode').value = 'none';
  document.getElementById('srv-port-check-msg').classList.add('hidden');
  document.getElementById('server-modal').classList.remove('hidden');
});

window.editServerByIndex = (idx) => {
  const s = state.overview?.servers?.[idx];
  if (!s) return;
  document.getElementById('server-modal-title').textContent = t('srv.edit_title', { section: s.section });
  document.getElementById('srv-orig-section').value = s.section;
  document.getElementById('srv-protocol').value = s.protocol;
  document.getElementById('srv-local-port').value = s.local_port;
  document.getElementById('srv-remote-host').value = s.server_address;
  document.getElementById('srv-remote-port').value = s.server_port;
  document.getElementById('srv-local-addr').value = s.local_address || '0.0.0.0';
  document.getElementById('srv-server-starttls').checked = !!s.server_starttls;
  document.getElementById('srv-local-tls').checked = !!(s.local_certificate_path && s.local_key_path);
  document.getElementById('srv-local-starttls').checked = !!s.local_starttls;
  document.getElementById('srv-port-mode').value = 'none';
  document.getElementById('srv-port-check-msg').classList.add('hidden');
  document.getElementById('server-modal').classList.remove('hidden');
};

document.getElementById('btn-check-srv-port').addEventListener('click', async () => {
  const port = parseInt(document.getElementById('srv-local-port').value, 10);
  const msgEl = document.getElementById('srv-port-check-msg');
  if (!port || port < 1024 || port > 65535) {
    msgEl.textContent = t('srv.port_invalid');
    msgEl.style.color = '#f87171';
    msgEl.classList.remove('hidden');
    return;
  }
  msgEl.textContent = t('srv.port_checking');
  msgEl.style.color = '';
  msgEl.classList.remove('hidden');
  try {
    const res = await api('/api/ports/check', {
      method: 'POST',
      body: JSON.stringify({ ports: [port] }),
    });
    const st = (res.statuses || []).find(x => x.port === port);
    if (!st) {
      msgEl.textContent = t('srv.port_checked');
      return;
    }
    msgEl.textContent = st.message;
    msgEl.style.color = (st.status === 'open_on_proxy' || st.status === 'available') ? '#4ade80' : '#f87171';
  } catch (err) {
    msgEl.textContent = err.message;
    msgEl.style.color = '#f87171';
  }
});

document.getElementById('server-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const useTLS = document.getElementById('srv-local-tls').checked;
  const certPath = useTLS ? (state.overview?.certs?.container_cert_path || '/config/proxy-cert.pem') : '';
  const keyPath = useTLS ? (state.overview?.certs?.container_key_path || '/config/proxy-key.pem') : '';
  try {
    const res = await api('/api/servers', {
      method: 'POST',
      body: JSON.stringify({
        section: document.getElementById('srv-orig-section').value,
        protocol: document.getElementById('srv-protocol').value,
        local_port: parseInt(document.getElementById('srv-local-port').value, 10),
        local_address: document.getElementById('srv-local-addr').value.trim(),
        server_address: document.getElementById('srv-remote-host').value.trim(),
        server_port: parseInt(document.getElementById('srv-remote-port').value, 10),
        server_starttls: document.getElementById('srv-server-starttls').checked,
        local_starttls: document.getElementById('srv-local-starttls').checked,
        local_certificate_path: certPath,
        local_key_path: keyPath,
        port_mode: document.getElementById('srv-port-mode').value,
      }),
    });
    closeModal('server-modal');
    toast(t('srv.saved', { section: res.section }), 'success');
    showReloadHint(res.reload);
    await loadOverview();
    await loadRawConfig();
  } catch (err) {
    toast(err.message, 'error');
  }
});

window.deleteServerByIndex = async (idx) => {
  const s = state.overview?.servers?.[idx];
  if (!s) return;
  const section = s.section;
  const ok = await confirmDialog({
    title: t('srv.delete_title'),
    message: t('srv.delete_msg', { section }),
    confirmLabel: t('dash.btn_remove'),
  });
  if (!ok) return;
  try {
    const res = await api(`/api/servers/${encodeURIComponent(section)}`, { method: 'DELETE' });
    toast(t('srv.deleted', { section }), 'success');
    showReloadHint(res.reload);
    await loadOverview();
    await loadRawConfig();
  } catch (err) {
    toast(err.message, 'error');
  }
};

// ==================== DOCKER & COMPOSE PORT MANAGER ====================
function syncPortModeSelects(dp) {
  const dockerOK = !!(dp && dp.docker_available && !dp.self_pid_shared);
  const composeOK = !!(dp && dp.compose_available);

  for (const selId of ['tpl-port-mode', 'srv-port-mode']) {
    const sel = document.getElementById(selId);
    if (!sel) continue;
    for (const opt of sel.options) {
      if (opt.value === 'docker_only') {
        opt.disabled = !dockerOK;
      } else if (opt.value === 'compose_and_docker') {
        opt.disabled = !(dockerOK && composeOK);
      } else if (opt.value === 'compose_only') {
        opt.disabled = !composeOK;
      }
    }
    if (sel.selectedOptions[0]?.disabled) {
      sel.value = 'none';
    }
  }
}

function renderPortsModal(dp, checkedPortsOverride) {
  const info = dp || state.overview?.docker_ports || {};
  const listEl = document.getElementById('ports-manager-list');
  const composeInfo = document.getElementById('ports-modal-compose-info');
  const pidWarn = document.getElementById('ports-modal-pid-warn');

  pidWarn.classList.toggle('hidden', !info.self_pid_shared);

  if (info.compose_available) {
    composeInfo.className = 'alert alert-info small';
    composeInfo.innerHTML = `<span>${t('ports.compose_active', {
      path: escapeHtml(info.compose_path),
      service: escapeHtml(info.compose_service || 'email-oauth2-proxy'),
    })}</span>`;
  } else {
    composeInfo.className = 'alert alert-warn small';
    const hint = info.compose_host_hint
      ? t('ports.compose_host_hint', { hint: escapeHtml(info.compose_host_hint) })
      : '';
    composeInfo.innerHTML = `<span>${t('ports.compose_inactive', { hint })}</span>`;
  }

  document.getElementById('btn-ports-apply-compose-only').disabled = !info.compose_available;
  document.getElementById('btn-ports-apply-docker').disabled = !info.docker_available || !!info.self_pid_shared;
  document.getElementById('btn-ports-apply-both').disabled = !info.docker_available || !info.compose_available || !!info.self_pid_shared;

  const statuses = info.statuses || [];
  if (statuses.length === 0) {
    listEl.innerHTML = `<div class="empty-state">${t('ports.empty')}</div>`;
    updatePortsYAMLPreview();
    return;
  }

  listEl.innerHTML = statuses.map((st) => {
    const isProtected = !!st.protected;
    const isChecked = isProtected
      ? true
      : (checkedPortsOverride ? checkedPortsOverride.has(st.port) : (st.published_on_proxy || !st.section?.startsWith('EXTRA-')));

    let statusCls = 'badge-muted';
    let statusTxt = st.status;
    if (st.status === 'open_on_proxy') {
      statusCls = 'badge-success';
      statusTxt = isProtected ? t('ports.status_protected') : t('ports.status_open');
    } else if (st.status === 'available') {
      statusCls = 'badge-info';
      statusTxt = t('ports.status_available');
    } else if (st.status === 'conflict_docker' || st.status === 'conflict_host' || st.status === 'conflict_config') {
      statusCls = 'badge-danger';
      statusTxt = t('ports.status_conflict');
    }

    return `
      <div class="port-row-item">
        <label class="checkbox">
          <input type="checkbox" class="port-mgr-cb" data-port="${st.port}" data-section="${escapeHtml(st.section || '')}" data-host="${escapeHtml(st.server_address || '')}" ${isChecked ? 'checked' : ''} ${isProtected ? 'disabled' : ''} onchange="updatePortsYAMLPreview()" />
          <span class="port-tag">${escapeHtml(st.section || `PORT-${st.port}`)}</span>
          <span class="badge ${statusCls}">${escapeHtml(statusTxt)}</span>
        </label>
        <div class="muted small">${escapeHtml(st.message || '')}</div>
      </div>
    `;
  }).join('');

  updatePortsYAMLPreview();
}

function getSelectedPortSpecsFromModal() {
  const specs = [];
  document.querySelectorAll('.port-mgr-cb').forEach(cb => {
    if (cb.disabled || !cb.checked) return;
    const port = parseInt(cb.dataset.port, 10);
    if (!port) return;
    specs.push({
      port,
      section: cb.dataset.section || `PORT-${port}`,
      server_address: cb.dataset.host || '',
    });
  });
  return specs;
}

function updatePortsYAMLPreview() {
  const specs = getSelectedPortSpecsFromModal().sort((a, b) => a.port - b.port);
  const el = document.getElementById('ports-yaml-preview');
  if (!el) return;
  if (specs.length === 0) {
    el.value = '    ports: []';
    return;
  }
  const lines = ['    ports:'];
  for (const s of specs) {
    const comment = s.server_address ? ` # ${s.section} (${s.server_address})` : (s.section ? ` # ${s.section}` : '');
    lines.push(`      - "${s.port}:${s.port}"${comment}`);
  }
  el.value = lines.join('\n');
}
window.updatePortsYAMLPreview = updatePortsYAMLPreview;

async function openPortsModal() {
  renderPortsModal(state.overview?.docker_ports);
  document.getElementById('ports-modal').classList.remove('hidden');
  await checkAllDockerPorts(true);
}
window.openPortsModal = openPortsModal;

async function checkAllDockerPorts(silent = false) {
  const btn = document.getElementById('btn-ports-check-all');
  btn.disabled = true;
  const selected = new Set(getSelectedPortSpecsFromModal().map(s => s.port));
  try {
    const res = await api('/api/ports/check', {
      method: 'POST',
      body: JSON.stringify({ ports: Array.from(selected) }),
    });
    if (state.overview && res.docker_ports) {
      state.overview.docker_ports = res.docker_ports;
    }
    renderPortsModal(res.docker_ports, selected.size ? selected : null);
    if (!silent) {
      toast(t('ports.checked_toast'), 'success');
    }
  } catch (err) {
    if (!silent) toast(err.message, 'error');
  } finally {
    btn.disabled = false;
  }
}

async function applyDockerPorts(mode) {
  const specs = getSelectedPortSpecsFromModal();
  const modeLabel = mode === 'compose_and_docker'
    ? t('ports.apply_msg_both')
    : (mode === 'docker_only' ? t('ports.apply_msg_docker') : t('ports.apply_msg_compose'));

  const ok = await confirmDialog({
    title: t('ports.apply_title'),
    message: t('ports.apply_confirm_msg', { action: modeLabel }),
    confirmLabel: t('ports.apply_btn'),
    danger: false,
  });
  if (!ok) return;

  const btns = ['btn-ports-apply-compose-only', 'btn-ports-apply-docker', 'btn-ports-apply-both'];
  btns.forEach(id => { document.getElementById(id).disabled = true; });

  try {
    const res = await api('/api/ports/apply', {
      method: 'POST',
      body: JSON.stringify({ mode, specs }),
    });
    toast(res.result?.message || t('ports.updated_toast'), 'success');
    closeModal('ports-modal');
    setTimeout(loadOverview, 1200);
  } catch (err) {
    toast(err.message, 'error');
    renderPortsModal(state.overview?.docker_ports);
  }
}

document.getElementById('btn-open-ports-modal').addEventListener('click', openPortsModal);
document.getElementById('btn-ports-check-all').addEventListener('click', () => checkAllDockerPorts(false));
document.getElementById('btn-copy-ports-yaml').addEventListener('click', () => {
  navigator.clipboard.writeText(document.getElementById('ports-yaml-preview').value).then(() => toast(t('ports.yaml_copied'), 'success'));
});
document.getElementById('btn-ports-apply-compose-only').addEventListener('click', () => applyDockerPorts('compose_only'));
document.getElementById('btn-ports-apply-docker').addEventListener('click', () => applyDockerPorts('docker_only'));
document.getElementById('btn-ports-apply-both').addEventListener('click', () => applyDockerPorts('compose_and_docker'));

// Configured Accounts Table & Full Edit Modal (all emailproxy.config flows)
const FLOW_LABELS = {
  '': 'Authorization Code',
  device: 'Device Flow (DAG)',
  client_credentials: 'Client Credentials (CCG)',
  password: 'Password (ROPCG)',
  service_account: 'Google Service Account',
};

function renderConfigAccountsTable(accounts) {
  const tbody = document.getElementById('config-accounts-tbody');
  if (!tbody) return;
  const cfgAccounts = accounts.filter(a => !a.from_cache_only);
  if (cfgAccounts.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" class="muted" style="text-align:center;padding:20px;">${escapeHtml(t('cfg.accounts_empty'))}</td></tr>`;
    return;
  }

  tbody.innerHTML = cfgAccounts.map(a => {
    const origIdx = accounts.indexOf(a);
    const flowLabel = FLOW_LABELS[a.oauth2_flow || ''] || a.oauth2_flow;
    const flags = [];
    if (a.has_client_secret) flags.push('<span class="badge badge-muted">Secret</span>');
    if (a.use_pkce) flags.push('<span class="badge badge-info">PKCE</span>');
    if (a.jwt_certificate_path) flags.push('<span class="badge badge-warn">JWT</span>');

    return `
      <tr>
        <td>
          <strong>${escapeHtml(a.account)}</strong>
          ${a.is_catch_all ? `<span class="badge badge-info" style="margin-left:6px;">Catch-All</span>` : ''}
        </td>
        <td><span class="badge badge-muted">${escapeHtml(flowLabel)}</span></td>
        <td>
          <div class="small muted" style="font-family:var(--mono);">${escapeHtml((a.client_id || '').slice(0, 26))}${(a.client_id || '').length > 26 ? '…' : ''}</div>
          ${flags.length ? `<div class="row" style="margin-top:4px;">${flags.join('')}</div>` : ''}
        </td>
        <td class="small muted" style="font-family:var(--mono);">${escapeHtml(a.redirect_uri || '—')}</td>
        <td class="right">
          <div class="row right">
            <button class="btn btn-secondary btn-xs" onclick="editAccountByIndex(${origIdx})">${escapeHtml(t('cfg.btn_edit'))}</button>
            <button class="btn btn-danger btn-xs" onclick="deleteAccountByIndex(${origIdx}, false)">${escapeHtml(t('dash.btn_remove'))}</button>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function updateAccountModalFlowUI() {
  const flow = document.getElementById('acc-flow').value;
  const isNonInteractive = ['client_credentials', 'password', 'service_account'].includes(flow);
  const isDevice = flow === 'device';
  const isServiceAcc = flow === 'service_account';

  document.getElementById('acc-perm-url-wrap').classList.toggle('hidden', isNonInteractive);
  document.getElementById('acc-redirect-wrap').classList.toggle('hidden', isNonInteractive || isDevice);
  document.getElementById('acc-resource-wrap').classList.toggle('hidden', flow !== 'password');
  document.getElementById('acc-client-id-hint').classList.toggle('hidden', !isServiceAcc);

  document.getElementById('acc-client-id-label').innerHTML = isServiceAcc
    ? t('acc.client_id_sa')
    : t('acc.client_id');

  document.getElementById('acc-secret-label').innerHTML = isServiceAcc
    ? t('acc.secret_label_sa')
    : t('acc.secret_label');

  document.getElementById('acc-client-secret').rows = isServiceAcc ? 6 : 2;
  document.getElementById('acc-client-secret').placeholder = isServiceAcc
    ? t('acc.secret_ph_sa')
    : t('acc.secret_ph');
}

document.getElementById('acc-flow').addEventListener('change', updateAccountModalFlowUI);
document.getElementById('btn-acc-use-panel').addEventListener('click', () => {
  document.getElementById('acc-redirect-uri').value = getDynamicPanelCallbackURL();
  if (!document.getElementById('acc-redirect-listen').value.trim()) {
    document.getElementById('acc-redirect-listen').value = 'http://0.0.0.0:80';
  }
});

document.getElementById('btn-add-account').addEventListener('click', () => {
  document.getElementById('account-modal-title').textContent = t('acc.new_title');
  document.getElementById('acc-orig-name').value = '';
  document.getElementById('acc-name').value = '';
  document.getElementById('acc-flow').value = '';
  document.getElementById('acc-perm-url').value = '';
  document.getElementById('acc-token-url').value = '';
  document.getElementById('acc-scope').value = '';
  document.getElementById('acc-resource').value = '';
  document.getElementById('acc-redirect-uri').value = getDynamicPanelCallbackURL();
  document.getElementById('acc-redirect-listen').value = 'http://0.0.0.0:80';
  document.getElementById('acc-client-id').value = '';
  document.getElementById('acc-client-secret').value = '';
  document.getElementById('acc-use-pkce').checked = false;
  document.getElementById('acc-jwt-cert').value = '';
  document.getElementById('acc-jwt-key').value = '';
  document.getElementById('acc-jwt-details').open = false;
  document.getElementById('acc-keep-secret-wrap').classList.add('hidden');
  document.getElementById('acc-keep-secret').checked = false;
  updateAccountModalFlowUI();
  document.getElementById('account-modal').classList.remove('hidden');
});

window.editAccountByIndex = (idx) => {
  const a = state.overview?.accounts?.[idx];
  if (!a) return;
  document.getElementById('account-modal-title').textContent = t('acc.edit_item_title', { account: a.account });
  document.getElementById('acc-orig-name').value = a.account;
  document.getElementById('acc-name').value = a.account;
  document.getElementById('acc-flow').value = a.oauth2_flow || '';
  document.getElementById('acc-perm-url').value = a.permission_url || '';
  document.getElementById('acc-token-url').value = a.token_url || '';
  document.getElementById('acc-scope').value = a.oauth2_scope || '';
  document.getElementById('acc-resource').value = a.oauth2_resource || '';
  document.getElementById('acc-redirect-uri').value = a.redirect_uri || '';
  document.getElementById('acc-redirect-listen').value = a.redirect_listen_address || '';
  document.getElementById('acc-client-id').value = a.client_id || '';
  document.getElementById('acc-client-secret').value = '';
  document.getElementById('acc-use-pkce').checked = !!a.use_pkce;
  document.getElementById('acc-jwt-cert').value = a.jwt_certificate_path || '';
  document.getElementById('acc-jwt-key').value = a.jwt_key_path || '';
  document.getElementById('acc-jwt-details').open = !!(a.jwt_certificate_path || a.jwt_key_path);
  document.getElementById('acc-keep-secret-wrap').classList.toggle('hidden', !a.has_client_secret);
  document.getElementById('acc-keep-secret').checked = !!a.has_client_secret;
  updateAccountModalFlowUI();
  document.getElementById('account-modal').classList.remove('hidden');
};

document.getElementById('account-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  try {
    const res = await api('/api/accounts', {
      method: 'POST',
      body: JSON.stringify({
        original_account: document.getElementById('acc-orig-name').value,
        account: document.getElementById('acc-name').value.trim(),
        oauth2_flow: document.getElementById('acc-flow').value,
        permission_url: document.getElementById('acc-perm-url').value.trim(),
        token_url: document.getElementById('acc-token-url').value.trim(),
        oauth2_scope: document.getElementById('acc-scope').value.trim(),
        oauth2_resource: document.getElementById('acc-resource').value.trim(),
        redirect_uri: document.getElementById('acc-redirect-uri').value.trim(),
        redirect_listen_address: document.getElementById('acc-redirect-listen').value.trim(),
        client_id: document.getElementById('acc-client-id').value.trim(),
        client_secret: document.getElementById('acc-client-secret').value.trim(),
        keep_existing_secret: document.getElementById('acc-keep-secret').checked,
        use_pkce: document.getElementById('acc-use-pkce').checked,
        jwt_certificate_path: document.getElementById('acc-jwt-cert').value.trim(),
        jwt_key_path: document.getElementById('acc-jwt-key').value.trim(),
      }),
    });
    closeModal('account-modal');
    toast(t('acc.saved', { account: res.account }), 'success');
    showReloadHint(res.reload);
    await loadOverview();
    await loadRawConfig();
  } catch (err) {
    toast(err.message, 'error');
  }
});

// Globals
function renderGlobals(g) {
  document.getElementById('glob-catchall').checked = !!g.allow_catch_all_accounts;
  document.getElementById('glob-delete-on-err').checked = !!g.delete_account_token_on_password_error;
  document.getElementById('glob-encrypt-secret').checked = !!g.encrypt_client_secret_on_first_use;
  document.getElementById('glob-login-pass-secret').checked = !!g.use_login_password_as_client_credentials_secret;
}

document.getElementById('globals-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  try {
    const res = await api('/api/globals', {
      method: 'POST',
      body: JSON.stringify({
        allow_catch_all_accounts: document.getElementById('glob-catchall').checked,
        delete_account_token_on_password_error: document.getElementById('glob-delete-on-err').checked,
        encrypt_client_secret_on_first_use: document.getElementById('glob-encrypt-secret').checked,
        use_login_password_as_client_credentials_secret: document.getElementById('glob-login-pass-secret').checked,
      }),
    });
    toast(t('cfg.globals_saved'), 'success');
    showReloadHint(res.reload);
    await loadOverview();
    await loadRawConfig();
  } catch (err) {
    toast(err.message, 'error');
  }
});

// Raw Editor
async function loadRawConfig() {
  try {
    const res = await api('/api/config/raw');
    document.getElementById('raw-config-editor').value = res.content || '';
  } catch (err) {
    toast(err.message, 'error');
  }
}

document.getElementById('btn-load-raw').addEventListener('click', loadRawConfig);
document.getElementById('btn-save-raw').addEventListener('click', async () => {
  try {
    const res = await api('/api/config/raw', {
      method: 'PUT',
      body: JSON.stringify({ content: document.getElementById('raw-config-editor').value }),
    });
    toast(t('cfg.raw_saved'), 'success');
    showReloadHint(res.reload);
    await loadOverview();
  } catch (err) {
    toast(err.message, 'error');
  }
});

// ==================== CERTIFICATES ====================
function renderCerts(c) {
  const badge = document.getElementById('cert-badge');
  const body = document.getElementById('cert-status-body');
  const dlBtn = document.getElementById('btn-download-ca');

  if (!c.has_ca && !c.has_server_cert) {
    badge.className = 'badge badge-warn';
    badge.textContent = t('certs.not_generated');
    dlBtn.classList.add('hidden');
    body.innerHTML = `<div class="empty-state">${t('certs.empty_state')}</div>`;
    if (!document.getElementById('cert-hosts').value && location.hostname !== 'localhost') {
      document.getElementById('cert-hosts').value = location.hostname;
    }
    return;
  }

  badge.className = 'badge badge-success';
  badge.textContent = t('certs.present');
  dlBtn.classList.toggle('hidden', !c.has_ca);

  const ips = (c.server_ips || []).map(ip => `<code>${escapeHtml(ip)}</code>`).join(' ');
  const dns = (c.server_dns || []).map(d => `<code>${escapeHtml(d)}</code>`).join(' ');

  body.innerHTML = `
    <div class="subcard">
      <div class="subcard-title">${t('certs.local_ca')}</div>
      <div><strong>Subject:</strong> ${escapeHtml(c.ca_subject || '—')}</div>
      <div><strong>${escapeHtml(t('certs.valid_until'))}</strong> ${c.ca_not_after ? new Date(c.ca_not_after).toLocaleDateString(getDateLocale()) : '—'}</div>
      <div class="muted" style="word-break:break-all;"><strong>SHA-256:</strong> ${escapeHtml(c.ca_fingerprint || '—')}</div>
    </div>
    <div class="subcard">
      <div class="subcard-title">${t('certs.proxy_cert')}</div>
      <div><strong>CN:</strong> ${escapeHtml(c.server_subject || '—')}</div>
      <div><strong>${escapeHtml(t('certs.valid_until'))}</strong> ${c.server_not_after ? new Date(c.server_not_after).toLocaleDateString(getDateLocale()) : '—'}</div>
      <div><strong>SAN IPs:</strong> ${ips || '—'}</div>
      <div><strong>SAN DNS:</strong> ${dns || '—'}</div>
    </div>
  `;

  if (!document.getElementById('cert-hosts').value) {
    const nonLoopback = (c.server_ips || []).filter(ip => ip !== '127.0.0.1');
    document.getElementById('cert-hosts').value = nonLoopback.join(', ') || location.hostname;
  }
}

document.getElementById('cert-gen-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const hostsRaw = document.getElementById('cert-hosts').value;
  try {
    const res = await api('/api/certs/generate', {
      method: 'POST',
      body: JSON.stringify({
        hosts: [hostsRaw],
        days: parseInt(document.getElementById('cert-days').value, 10) || 3650,
        reuse_ca: document.getElementById('cert-reuse-ca').checked,
        apply_to_all_servers: document.getElementById('cert-apply-all').checked,
      }),
    });
    toast(t('certs.gen_success'), 'success');
    if (res.reload) showReloadHint(res.reload);
    await loadOverview();
  } catch (err) {
    toast(err.message, 'error');
  }
});

document.getElementById('btn-enable-tls-all').addEventListener('click', () => toggleAllTLS(true));
document.getElementById('btn-disable-tls-all').addEventListener('click', () => toggleAllTLS(false));

async function toggleAllTLS(enable) {
  try {
    const res = await api('/api/certs/apply-tls', {
      method: 'POST',
      body: JSON.stringify({ enable }),
    });
    toast(enable ? t('certs.tls_enabled_all') : t('certs.tls_disabled_all'), 'success');
    showReloadHint(res.reload);
    await loadOverview();
  } catch (err) {
    toast(err.message, 'error');
  }
}

// ==================== LIVE LOGS (SSE) ====================
function connectLogStream() {
  disconnectLogStream();
  state.logLines = [];
  document.getElementById('logs-output').innerHTML = '';

  const badge = document.getElementById('logs-sse-badge');
  badge.className = 'badge badge-info';
  state.logStatusKey = 'logs.connecting';
  badge.textContent = t(state.logStatusKey);

  const es = new EventSource('/api/logs/stream');
  state.logSource = es;

  es.onopen = () => {
    badge.className = 'badge badge-success';
    state.logStatusKey = 'logs.live';
    badge.textContent = t(state.logStatusKey);
  };

  es.onmessage = (ev) => {
    if (state.logsPaused) return;
    try {
      const line = JSON.parse(ev.data);
      state.logLines.push(line);
      if (state.logLines.length > 2000) state.logLines.shift();
      appendLogLine(line);
    } catch (_) {}
  };

  es.onerror = () => {
    badge.className = 'badge badge-warn';
    state.logStatusKey = 'logs.reconnecting';
    badge.textContent = t(state.logStatusKey);
  };
}

function disconnectLogStream() {
  if (state.logSource) {
    state.logSource.close();
    state.logSource = null;
  }
  state.logStatusKey = 'logs.disconnected';
  const badge = document.getElementById('logs-sse-badge');
  if (badge) {
    badge.className = 'badge badge-muted';
    badge.textContent = t(state.logStatusKey);
  }
}

function classifyLogPayload(text) {
  const low = text.toLowerCase();
  if (low.includes('error') || low.includes('failed') || low.includes('bad request') || low.includes('exception') || /\b(no|bad)\s+(login|authenticate)/i.test(text)) {
    return 'log-err';
  }
  if (low.includes('ok authenticate') || low.includes('ok success') || low.includes('successfully authenticated') || low.includes('initialised email oauth') || low.includes('handshake complete')) {
    return 'log-ok';
  }
  if (low.includes('please visit the following url') || low.includes('http://') || low.includes('https://')) {
    return 'log-url';
  }
  if (/^\[.+\]$/.test(text.trim())) {
    return 'log-dim';
  }
  return '';
}

function cleanPythonBytesPayload(payload) {
  let s = payload.trim();
  if ((s.startsWith("b'") && s.endsWith("'")) || (s.startsWith('b"') && s.endsWith('"'))) {
    const quote = s[1];
    s = s.slice(2, -1);
    if (quote === "'") s = s.replace(/\\'/g, "'");
    if (quote === '"') s = s.replace(/\\"/g, '"');
  } else {
    s = s.replace(/\\r\\n['"]?$/, '');
  }
  s = s.replace(/\\r\\n/g, '\n').replace(/\\n/g, '\n').replace(/\\r/g, '').replace(/\\t/g, '  ').replace(/\\\\/g, '\\');
  const parts = s.split('\n').map(p => p.trim()).filter(Boolean);
  return parts;
}

function formatLogLineHTML(raw) {
  if (document.getElementById('logs-raw')?.checked) {
    const cls = classifyLogPayload(raw);
    return `<span class="${cls}">${escapeHtml(raw)}</span>`;
  }

  let tsFull = '';
  let tsShort = '';
  let rest = raw;
  const tsMatch = raw.match(/^(\d{4}-\d{2}-\d{2}\s+(\d{2}:\d{2}:\d{2})(?:,\d+)?):\s*(.*)$/);
  if (tsMatch) {
    tsFull = tsMatch[1];
    tsShort = tsMatch[2];
    rest = tsMatch[3];
  }

  const tsSpan = tsShort ? `<span class="log-ts" title="${escapeHtml(tsFull)}">${escapeHtml(tsShort)}</span>` : '';

  const protoMatch = rest.match(/^(IMAP|SMTP|POP)\s+\(([^)]+)\)\s+(?:(-->|<--|<->)\s+)?(.*)$/);
  if (protoMatch) {
    const proto = protoMatch[1];
    const connRaw = protoMatch[2];
    const dirRaw = protoMatch[3] || '';
    const bodyRaw = protoMatch[4];

    const [routePart, accountPart] = connRaw.split(';').map(x => x.trim());
    const remoteHost = routePart.includes('}-') ? routePart.split('}-').pop() : routePart;
    const metaLabel = accountPart ? `${remoteHost} · ${accountPart}` : remoteHost;

    const protoCls = proto === 'IMAP' ? 'log-proto-imap' : (proto === 'SMTP' ? 'log-proto-smtp' : 'log-proto-pop');
    const dirSpan = dirRaw
      ? `<span class="log-dir ${dirRaw === '<--' ? 'log-dir-in' : (dirRaw === '-->' ? 'log-dir-out' : '')}">${dirRaw === '<--' ? '←' : (dirRaw === '-->' ? '→' : '↔')}</span>`
      : '';

    const parts = cleanPythonBytesPayload(bodyRaw);
    const joinedText = parts.join(' | ');
    const payloadCls = classifyLogPayload(joinedText);
    const renderedBody = parts.length > 1
      ? parts.map(p => `<span class="${classifyLogPayload(p) || payloadCls}">${escapeHtml(p)}</span>`).join('<span class="log-sep">|</span>')
      : `<span class="${payloadCls}">${escapeHtml(joinedText || bodyRaw)}</span>`;

    return `${tsSpan}<span class="log-proto ${protoCls}">${proto}</span><span class="log-meta" title="${escapeHtml(connRaw)}">${escapeHtml(metaLabel)}</span>${dirSpan}${renderedBody}`;
  }

  const parts = cleanPythonBytesPayload(rest);
  const cleanText = parts.length > 0 ? parts.join(' | ') : rest;
  const cls = classifyLogPayload(cleanText);
  return `${tsSpan}<span class="${cls}">${escapeHtml(cleanText)}</span>`;
}

function appendLogLine(line) {
  const filter = document.getElementById('logs-filter').value.trim().toLowerCase();
  if (filter && !line.toLowerCase().includes(filter)) return;
  const out = document.getElementById('logs-output');
  const div = document.createElement('div');
  div.className = 'log-line';
  div.innerHTML = formatLogLineHTML(line);
  out.appendChild(div);
  if (document.getElementById('logs-autoscroll').checked) {
    out.scrollTop = out.scrollHeight;
  }
}

function rerenderLogs() {
  const out = document.getElementById('logs-output');
  out.innerHTML = '';
  const filter = document.getElementById('logs-filter').value.trim().toLowerCase();
  const frag = document.createDocumentFragment();
  for (const line of state.logLines) {
    if (filter && !line.toLowerCase().includes(filter)) continue;
    const div = document.createElement('div');
    div.className = 'log-line';
    div.innerHTML = formatLogLineHTML(line);
    frag.appendChild(div);
  }
  out.appendChild(frag);
  if (document.getElementById('logs-autoscroll').checked) {
    out.scrollTop = out.scrollHeight;
  }
}

document.getElementById('logs-filter').addEventListener('input', rerenderLogs);
document.getElementById('logs-raw').addEventListener('change', rerenderLogs);
document.getElementById('btn-logs-toggle').addEventListener('click', (e) => {
  state.logsPaused = !state.logsPaused;
  e.target.textContent = t(state.logsPaused ? 'logs.resume' : 'logs.pause');
});
document.getElementById('btn-logs-clear').addEventListener('click', async () => {
  const ok = await confirmDialog({
    title: t('logs.clear_title'),
    message: t('logs.clear_msg'),
    confirmLabel: t('logs.clear'),
  });
  if (!ok) return;
  try {
    await api('/api/logs/clear', { method: 'POST' });
    state.logLines = [];
    document.getElementById('logs-output').innerHTML = '';
    toast(t('logs.cleared'), 'success');
  } catch (err) {
    toast(err.message, 'error');
  }
});

// ==================== GLOBAL ACTIONS ====================
document.querySelectorAll('.nav-item').forEach(btn => {
  btn.addEventListener('click', () => switchTab(btn.dataset.tab));
});

document.getElementById('btn-refresh').addEventListener('click', async () => {
  try {
    await loadOverview();
    toast(t('common.status_updated'), 'success');
  } catch (err) {
    toast(err.message, 'error');
  }
});

document.getElementById('btn-reload-proxy').addEventListener('click', restartProxyNow);

document.getElementById('btn-toggle-debug').addEventListener('click', async () => {
  const btn = document.getElementById('btn-toggle-debug');
  btn.disabled = true;
  try {
    const res = await api('/api/proxy/toggle-debug', { method: 'POST' });
    renderDebugButton(res.debug_mode, res.debug_known);
    toast(res.message || t('common.debug_updated'), res.reloaded ? 'success' : 'info');
  } catch (err) {
    toast(err.message, 'error');
  } finally {
    btn.disabled = false;
  }
});

async function bootstrap() {
  try {
    await Promise.all([loadTemplates(), loadOverview()]);
    startPendingPolling();
  } catch (err) {
    toast(err.message, 'error');
  }
}

(async () => {
  initLanguage();
  const ok = await checkAuth();
  if (ok) {
    await bootstrap();
  }
})();
