// SPDX-License-Identifier: GPL-2.0-only

const api = window.anygame;

const ICONS = {
  grid: '<rect x="3.5" y="3.5" width="7" height="7" rx="1.6"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.6"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.6"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.6"/>',
  cpu: '<rect x="7" y="7" width="10" height="10" rx="2.2"/><path d="M9.5 3v4M14.5 3v4M9.5 17v4M14.5 17v4M3 9.5h4M3 14.5h4M17 9.5h4M17 14.5h4"/>',
  settings: '<circle cx="12" cy="12" r="3.1"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06A1.7 1.7 0 0 0 15 19.36a1.7 1.7 0 0 0-1 1.55V21a2 2 0 1 1-4 0v-.09A1.7 1.7 0 0 0 9 19.36a1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.7 1.7 0 0 0 4.64 15a1.7 1.7 0 0 0-1.55-1H3a2 2 0 1 1 0-4h.09A1.7 1.7 0 0 0 4.64 9a1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.7 1.7 0 0 0 9 4.64h.09A1.7 1.7 0 0 0 10 3.09V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1 1.55 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.7 1.7 0 0 0 19.36 9v.09a1.7 1.7 0 0 0 1.55 1H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.51 1Z"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5.5M12 7.6v.1"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  search: '<circle cx="10.5" cy="10.5" r="6.5"/><path d="m15.5 15.5 4.5 4.5"/>',
  play: '<path d="M8 5.5 18.5 12 8 18.5v-13Z" fill="currentColor" stroke="none"/>',
  folder: '<path d="M3.5 7.5A2 2 0 0 1 5.5 5.5h4l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2v-10Z"/>',
  trash: '<path d="M4 7h16M9.5 7V5.5A1.5 1.5 0 0 1 11 4h2a1.5 1.5 0 0 1 1.5 1.5V7M6.5 7l.8 12.2A1.8 1.8 0 0 0 9.1 21h5.8a1.8 1.8 0 0 0 1.8-1.8L17.5 7M10 11v6M14 11v6"/>',
  refresh: '<path d="M20 11.5A8 8 0 1 0 18.4 16M20 5.5v6h-6"/>',
  download: '<path d="M12 4v11M7.5 10.5 12 15l4.5-4.5M5 19.5h14"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
  alert: '<path d="M12 4.5 21 20H3L12 4.5Z"/><path d="M12 10v4.2M12 17.2v.1"/>',
  x: '<path d="m6 6 12 12M18 6 6 18"/>',
  chevron: '<path d="m9.5 6 6 6-6 6"/>',
  terminal: '<rect x="3" y="4.5" width="18" height="15" rx="2.4"/><path d="m7.5 9.5 3 3-3 3M13 15h4"/>',
  monitor: '<rect x="3" y="4.5" width="18" height="12" rx="2.2"/><path d="M9 20.5h6M12 16.5v4"/>',
  external: '<path d="M14 4.5h5.5V10M19.5 4.5 11 13M18 14v4.5a2 2 0 0 1-2 2H5.5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2H10"/>',
  package: '<path d="m12 3 8 4.5v9L12 21l-8-4.5v-9L12 3Z"/><path d="m4.5 7.8 7.5 4.3 7.5-4.3M12 12.1V21"/>',
  gamepad: '<path d="M7.5 8.5h9a4.5 4.5 0 0 1 4.4 5.4l-.7 3.5a2.4 2.4 0 0 1-4 1.3l-1.6-1.4a2 2 0 0 0-1.3-.5h-2.6a2 2 0 0 0-1.3.5L8.8 18.7a2.4 2.4 0 0 1-4-1.3l-.7-3.5A4.5 4.5 0 0 1 7.5 8.5Z"/><path d="M8 11v3M6.5 12.5h3M15.5 11.5v.1M17.5 13.5v.1"/>',
  spark: '<path d="m12 3 1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8L12 3Z"/>',
  disk: '<ellipse cx="12" cy="6.5" rx="7.5" ry="3"/><path d="M4.5 6.5v11c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-11M4.5 12c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3"/>',
};

function icon(name, size = 18) {
  const body = ICONS[name] || '';
  return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${body}</svg>`;
}

function escapeHtml(value) {
  return String(value ?? '').replace(/[&<>"']/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[character]);
}

function escapeAttr(value) {
  return escapeHtml(value).replace(/`/g, '&#96;');
}

function fmtBytes(value) {
  if (!value) return '—';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let index = 0;
  let number = Number(value);
  while (number >= 1024 && index < units.length - 1) { number /= 1024; index++; }
  return `${number.toFixed(number >= 100 || index === 0 ? 0 : 1)} ${units[index]}`;
}

function timeAgo(value) {
  if (!value) return 'never';
  const then = new Date(value).getTime();
  if (!Number.isFinite(then)) return 'never';
  const seconds = Math.max(1, Math.round((Date.now() - then) / 1000));
  if (seconds < 60) return 'just now';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.round(hours / 24);
  if (days < 30) return `${days} d ago`;
  return new Date(value).toLocaleDateString();
}

const state = {
  info: null,
  settings: null,
  games: [],
  runtimes: [],
  doctor: [],
  updateInfo: null,
  route: 'library',
  selectedId: null,
  search: '',
  sort: 'name',
  filterTarget: 'all',
  jobHandlers: new Map(),
  booted: false,
};

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

function toast(title, text = '', kind = 'info', ms = 3800) {
  const root = $('#toast-root');
  const node = document.createElement('div');
  node.className = `toast ${kind}`;
  node.innerHTML = `<span class="bar"></span><div class="body"><div class="title">${escapeHtml(title)}</div>${text ? `<div class="text">${escapeHtml(text)}</div>` : ''}</div>`;
  root.appendChild(node);
  setTimeout(() => {
    node.style.transition = 'opacity .25s, transform .25s';
    node.style.opacity = '0';
    node.style.transform = 'translateY(6px)';
    setTimeout(() => node.remove(), 260);
  }, ms);
}

function statusMeta(game) {
  if (game.status === 'ready') return { cls: 'ok', label: 'Ready' };
  if (game.status === 'missing') return { cls: 'err', label: 'Missing package' };
  if (game.status === 'building') return { cls: 'info', label: 'Building' };
  return { cls: 'warn', label: 'Not converted' };
}

function targetLabel(value) {
  if (!value) return 'any';
  return value === 'windows' ? 'Windows' : 'Linux';
}

function monogram(name) {
  const letters = String(name || '?').replace(/[^A-Za-z0-9]/g, '');
  return (letters[0] || '?').toUpperCase();
}

function coverArt(game) {
  if (game.iconUrl) return `<img class="cover-art" src="${escapeAttr(game.iconUrl)}" alt="" draggable="false">`;
  return `<div class="cover-monogram">${escapeHtml(monogram(game.name))}</div>`;
}

function coverBlur(game) {
  if (!game.iconUrl) return '';
  return `<img class="cover-blur" src="${escapeAttr(game.iconUrl)}" alt="" aria-hidden="true" draggable="false">`;
}

function activeRuntimes() {
  const byTarget = {};
  for (const runtime of state.runtimes) {
    const current = byTarget[runtime.Target];
    if (!current || compareVersions(runtime.Version, current.Version) > 0) byTarget[runtime.Target] = runtime;
  }
  return byTarget;
}

function compareVersions(a, b) {
  const left = String(a || '').split('.').map((part) => parseInt(part, 10) || 0);
  const right = String(b || '').split('.').map((part) => parseInt(part, 10) || 0);
  for (let index = 0; index < Math.max(left.length, right.length); index++) {
    const delta = (left[index] || 0) - (right[index] || 0);
    if (delta !== 0) return delta;
  }
  return 0;
}

/* ------------------------------------------------------------------ */
/* Boot                                                                */
/* ------------------------------------------------------------------ */

document.addEventListener('DOMContentLoaded', boot);

async function boot() {
  $$('[data-icon]').forEach((node) => { node.innerHTML = icon(node.dataset.icon); });
  wireWindow();
  wireStaticActions();
  if (!api) {
    document.body.innerHTML = '<div style="padding:40px;font-family:sans-serif;color:#fff">AnyGame must run inside Electron.</div>';
    return;
  }
  try {
    state.info = await api.appInfo();
    state.settings = state.info.settings;
  } catch (error) {
    toast('Could not start', error.message, 'err', 8000);
  }
  await Promise.all([refreshLibrary(true), refreshRuntimes(), refreshDoctor()]);
  state.booted = true;
  setRoute('library');
  updateStatusBar();
}

async function refreshLibrary(quiet = false) {
  try {
    const data = await api.library.list();
    state.games = data.games || [];
  } catch (error) {
    if (!quiet) toast('Library error', error.message, 'err');
    state.games = [];
  }
  $('#nav-count').textContent = String(state.games.length);
  renderSidebarRuntime();
  if (state.route === 'library') renderLibrary();
}

async function refreshRuntimes(quiet = false) {
  try {
    state.runtimes = await api.runtime.list();
  } catch (error) {
    if (!quiet) toast('Runtime error', error.message, 'err');
    state.runtimes = [];
  }
  renderSidebarRuntime();
  if (state.route === 'runtimes') renderRuntimes();
  updateStatusBar();
}

async function refreshDoctor() {
  try {
    state.doctor = await api.doctor();
  } catch (_) {
    state.doctor = [];
  }
  updateStatusBar();
  if (state.route === 'settings') renderSettings();
}

function renderSidebarRuntime() {
  const byTarget = activeRuntimes();
  const linux = byTarget.linux;
  const windows = byTarget.windows;
  const dot = $('#runtime-dot');
  const title = $('#runtime-title');
  const sub = $('#runtime-sub');
  if (linux || windows) {
    dot.className = 'runtime-dot ok';
    const versions = [...new Set([linux?.Version, windows?.Version].filter(Boolean))];
    title.textContent = `AnyPS5 ${versions.join(' / ')}`;
    const targets = [linux && 'Linux', windows && 'Windows'].filter(Boolean).join(' + ');
    sub.textContent = targets ? `${targets} ready` : 'runtime ready';
  } else {
    dot.className = 'runtime-dot';
    title.textContent = 'No AnyPS5 runtime';
    sub.textContent = 'Open Runtimes to install';
  }
}

function updateStatusBar() {
  const byTarget = activeRuntimes();
  const runtime = byTarget.linux || byTarget.windows;
  $('#status-runtime').textContent = runtime ? `AnyPS5 ${runtime.Version} · ${runtime.Target}` : 'AnyPS5 · not installed';
  const gpu = state.doctor.find((check) => check.name === 'vulkan');
  $('#status-gpu').textContent = gpu ? `Vulkan · ${gpu.status === 'ok' ? 'ready' : gpu.status}` : 'GPU · unknown';
  if (state.info) {
    $('#status-home').textContent = state.info.home;
    $('#status-home').title = state.info.home;
  }
}

function wireWindow() {
  $$('[data-win]').forEach((button) => {
    button.addEventListener('click', () => {
      const action = button.dataset.win;
      if (action === 'minimize') api.window.minimize();
      if (action === 'toggle') api.window.toggle();
      if (action === 'close') api.window.close();
    });
  });
  api.window.onState(({ maximized }) => {
    const button = $('[data-win="toggle"]');
    if (!button) return;
    button.innerHTML = maximized
      ? '<svg viewBox="0 0 12 12"><rect x="3.2" y="4.4" width="4.6" height="4.6" rx="0.8"/><path d="M4.6 3.2h4.2v4.2"/></svg>'
      : '<svg viewBox="0 0 12 12"><rect x="3" y="3" width="6" height="6" rx="1"/></svg>';
  });
}

function wireStaticActions() {
  $$('.nav-item').forEach((item) => {
    item.addEventListener('click', () => setRoute(item.dataset.route));
  });
  $('#runtime-card').addEventListener('click', () => setRoute('runtimes'));
  document.addEventListener('contextmenu', (event) => event.preventDefault());
  document.addEventListener('keydown', (event) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f' && state.route === 'library') {
      event.preventDefault();
      $('#library-search')?.focus();
    }
    if (event.key === 'Escape') {
      if ($('#modal-root').childElementCount) closeModal();
      else if ($('#drawer-root').childElementCount) closeDrawer();
    }
  });
}

/* ------------------------------------------------------------------ */
/* Routing                                                             */
/* ------------------------------------------------------------------ */

function setRoute(route) {
  state.route = route;
  $$('.nav-item').forEach((item) => item.classList.toggle('active', item.dataset.route === route));
  renderHeader();
  if (route === 'library') renderLibrary();
  if (route === 'runtimes') { renderRuntimes(); refreshRuntimes(true); }
  if (route === 'settings') renderSettings();
  if (route === 'about') renderAbout();
}

function renderHeader() {
  const header = $('#page-header');
  const body = $('#page-body');
  body.innerHTML = '';
  if (state.route === 'library') {
    header.innerHTML = `
      <div class="page-heading">
        <h1 class="page-title">Games</h1>
        <p class="page-subtitle">Your converted PS5 library, powered by AnyPS5.</p>
      </div>
      <div class="page-actions">
        <label class="search">
          ${icon('search', 15)}
          <input id="library-search" type="text" placeholder="Search titles or IDs" value="${escapeAttr(state.search)}" autocomplete="off">
        </label>
        <select class="select" id="library-filter">
          <option value="all">All platforms</option>
          <option value="ready">Ready</option>
          <option value="pending">Not converted</option>
        </select>
        <button class="btn primary" id="add-game">${icon('plus', 16)} Add game</button>
      </div>`;
    $('#library-search').addEventListener('input', (event) => {
      state.search = event.target.value;
      renderLibrary();
    });
    $('#library-filter').value = state.filterTarget;
    $('#library-filter').addEventListener('change', (event) => {
      state.filterTarget = event.target.value;
      renderLibrary();
    });
    $('#add-game').addEventListener('click', addGame);
  } else if (state.route === 'runtimes') {
    header.innerHTML = `
      <div class="page-heading">
        <h1 class="page-title">Runtimes</h1>
        <p class="page-subtitle">AnyPS5 builds and system libraries used for conversions.</p>
      </div>
      <div class="page-actions">
        <button class="btn" id="check-updates">${icon('refresh', 16)} Check updates</button>
        <button class="btn primary" id="update-all">${icon('download', 16)} Install latest</button>
      </div>`;
    $('#check-updates').addEventListener('click', checkUpdates);
    $('#update-all').addEventListener('click', () => updateRuntime('all'));
  } else if (state.route === 'settings') {
    header.innerHTML = `
      <div class="page-heading">
        <h1 class="page-title">Settings</h1>
        <p class="page-subtitle">Conversion defaults, environment and diagnostics.</p>
      </div>
      <div class="page-actions">
        <button class="btn" id="open-data">${icon('folder', 16)} Open data folder</button>
      </div>`;
    $('#open-data').addEventListener('click', () => api.shell.openPath(state.info.home).catch((error) => toast('Could not open', error.message, 'err')));
  } else if (state.route === 'about') {
    header.innerHTML = `
      <div class="page-heading">
        <h1 class="page-title">About</h1>
        <p class="page-subtitle">A desktop launcher for AnyPS5.</p>
      </div>`;
  }
}

/* ------------------------------------------------------------------ */
/* Library                                                             */
/* ------------------------------------------------------------------ */

function filteredGames() {
  const needle = state.search.trim().toLowerCase();
  let games = state.games.slice();
  if (needle) {
    games = games.filter((game) =>
      String(game.name || '').toLowerCase().includes(needle) ||
      String(game.titleId || '').toLowerCase().includes(needle));
  }
  if (state.filterTarget === 'ready') games = games.filter((game) => game.status === 'ready');
  if (state.filterTarget === 'pending') games = games.filter((game) => game.status !== 'ready');
  const compare = {
    name: (a, b) => String(a.name).localeCompare(String(b.name)),
    recent: (a, b) => String(b.lastBuiltAt || b.addedAt || '').localeCompare(String(a.lastBuiltAt || a.addedAt || '')),
    status: (a, b) => String(a.status).localeCompare(String(b.status)) || String(a.name).localeCompare(String(b.name)),
  }[state.sort] || ((a, b) => 0);
  return games.sort(compare);
}

function renderLibrary() {
  if (state.route !== 'library') return;
  const body = $('#page-body');
  const games = filteredGames();
  const total = state.games.length;

  if (total === 0) {
    body.innerHTML = emptyState();
    $('#empty-add')?.addEventListener('click', addGame);
    $('#empty-refresh')?.addEventListener('click', () => refreshLibrary());
    return;
  }

  body.innerHTML = `
    <div class="library-toolbar">
      <span class="result-count">${games.length} of ${total} titles</span>
      <span class="spacer"></span>
      <select class="select" id="library-sort" title="Sort">
        <option value="name">Name A–Z</option>
        <option value="recent">Recently built</option>
        <option value="status">Status</option>
      </select>
    </div>
    <div class="grid">
      ${games.map(gameCard).join('')}
    </div>`;
  $('#library-sort').value = state.sort;
  $('#library-sort').addEventListener('change', (event) => {
    state.sort = event.target.value;
    renderLibrary();
  });
  $$('.game-card', body).forEach((card) => {
    const game = state.games.find((item) => item.id === card.dataset.id);
    if (!game) return;
    card.addEventListener('click', (event) => {
      if (event.target.closest('.cover-play')) {
        playGame(game);
        return;
      }
      openDrawer(game);
    });
    card.addEventListener('contextmenu', (event) => {
      event.preventDefault();
      openDrawer(game);
    });
  });
}

function emptyState() {
  return `
    <div class="empty">
      <div class="empty-art">
        <svg viewBox="0 0 120 120" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round">
          <rect x="16" y="28" width="88" height="64" rx="12"/>
          <path d="M16 44h88"/>
          <path d="M38 62h14M45 55v14"/>
          <circle cx="76" cy="62" r="5"/>
          <circle cx="88" cy="72" r="5"/>
          <path d="M60 44v48" stroke-dasharray="3 6"/>
        </svg>
      </div>
      <h2>Your library is empty</h2>
      <p>Add an extracted PS5 title and AnyGame will detect its metadata, convert the eboot with AnyPS5 and assemble a ready-to-play package.</p>
      <div class="empty-actions">
        <button class="btn primary" id="empty-add">${icon('plus', 16)} Add your first game</button>
        <button class="btn ghost" id="empty-refresh">${icon('refresh', 16)} Refresh</button>
      </div>
    </div>`;
}

function gameCard(game) {
  const status = statusMeta(game);
  const target = targetLabel(game.packageTarget || game.target);
  return `
    <article class="game-card ${game.id === state.selectedId ? 'selected' : ''}" data-id="${escapeAttr(game.id)}" title="${escapeAttr(game.name)}">
      <div class="cover">
        ${coverBlur(game)}
        <div class="cover-shade"></div>
        ${coverArt(game)}
        <div class="cover-top">
          <span class="chip ${status.cls}"><span class="dot"></span>${status.label}</span>
          <span class="chip platform">${escapeHtml(target)}</span>
        </div>
        <div class="cover-play"><span>${icon('play', 22)}</span></div>
      </div>
      <div class="card-body">
        <div class="card-title">${escapeHtml(game.name)}</div>
        <div class="card-meta">
          <span>${escapeHtml(game.titleId || 'no id')}</span>
          <span>·</span>
          <span class="target">${escapeHtml(target)}</span>
        </div>
      </div>
    </article>`;
}

async function addGame() {
  try {
    const added = await api.library.add();
    if (!added) return;
    await refreshLibrary(true);
    state.selectedId = added.id;
    toast('Game added', added.name, 'ok');
    openDrawer(added);
  } catch (error) {
    toast('Could not add the game', error.message, 'err', 7000);
  }
}

/* ------------------------------------------------------------------ */
/* Drawer                                                              */
/* ------------------------------------------------------------------ */

function openDrawer(game) {
  state.selectedId = game.id;
  $$('.game-card').forEach((card) => card.classList.toggle('selected', card.dataset.id === game.id));
  const root = $('#drawer-root');
  const status = statusMeta(game);
  const target = game.packageTarget || game.target || state.settings.target;
  const manifest = game.manifest;
  const diagnosis = manifest && manifest.diagnosis ? manifest.diagnosis : null;
  const warnings = Array.isArray(game.warnings) ? game.warnings : [];
  root.innerHTML = `
    <div class="drawer-backdrop" data-close-drawer></div>
    <aside class="drawer" role="dialog" aria-label="${escapeAttr(game.name)}">
      <div class="drawer-head">
        <span class="chip ${status.cls}"><span class="dot"></span>${status.label}</span>
        <span class="chip">${escapeHtml(targetLabel(target))}</span>
        <span class="spacer"></span>
        <button class="btn icon ghost" data-close-drawer title="Close">${icon('x', 16)}</button>
      </div>
      <div class="drawer-body">
        <div class="drawer-hero">
          ${coverBlur(game)}
          <div class="cover-shade"></div>
          ${coverArt(game)}
        </div>
        <h2 class="drawer-title">${escapeHtml(game.name)}</h2>
        <div class="drawer-sub">
          <span>${escapeHtml(game.titleId || 'no title id')}</span>
          ${game.contentVersion ? `<span>·</span><span>v${escapeHtml(game.contentVersion)}</span>` : ''}
          <span>·</span>
          <span>${escapeHtml(String(game.kind || '').toUpperCase() || 'unknown')}</span>
        </div>
        <div class="drawer-actions">
          <button class="btn primary" id="drawer-play" ${game.status === 'ready' ? '' : 'disabled'}>${icon('play', 15)} Play</button>
          <button class="btn" id="drawer-build">${icon(game.status === 'ready' ? 'refresh' : 'spark', 15)} ${game.status === 'ready' ? 'Rebuild' : 'Convert'}</button>
          <button class="btn icon" id="drawer-test" title="Run a diagnostic" ${game.status === 'ready' ? '' : 'disabled'}>${icon('terminal', 16)}</button>
          <button class="btn icon" id="drawer-folder" title="Open folder">${icon('folder', 16)}</button>
        </div>
        <div class="detail-list">
          <div class="detail-row"><span class="key">Source</span><span class="value mono">${escapeHtml(game.gamePath || '—')}</span></div>
          <div class="detail-row"><span class="key">Package</span><span class="value mono">${escapeHtml(game.packagePath || 'not converted yet')}</span></div>
          <div class="detail-row"><span class="key">Target</span><span class="value">${escapeHtml(targetLabel(target))}</span></div>
          <div class="detail-row"><span class="key">Runtime</span><span class="value">${escapeHtml(game.runtimeVersion ? `AnyPS5 ${game.runtimeVersion}` : '—')}</span></div>
          <div class="detail-row"><span class="key">Built</span><span class="value">${escapeHtml(timeAgo(game.lastBuiltAt))}</span></div>
          <div class="detail-row"><span class="key">Added</span><span class="value">${escapeHtml(timeAgo(game.addedAt))}</span></div>
          ${manifest && Array.isArray(manifest.libraries) ? `<div class="detail-row"><span class="key">Libraries</span><span class="value">${manifest.libraries.length} system libraries</span></div>` : ''}
          ${diagnosis ? `<div class="detail-row"><span class="key">Diagnosis</span><span class="value"><span class="chip ${diagnosis.verdict === 'running' || diagnosis.verdict === 'exited' ? 'ok' : diagnosis.verdict === 'missing-libraries' || diagnosis.verdict === 'crashed' ? 'err' : 'warn'}"><span class="dot"></span>${escapeHtml(diagnosisLabel(diagnosis.verdict))}</span> <span style="color:var(--faint)">${escapeHtml(timeAgo(diagnosis.at))}</span></span></div>` : ''}
          ${game.packagePath ? `<div class="detail-row"><span class="key">Run log</span><span class="value"><button class="btn small" id="drawer-log">${icon('terminal', 13)} Open log</button> <button class="btn small" id="drawer-verify">${icon('check', 13)} Verify</button></span></div>` : ''}
        </div>
        ${warnings.length ? `<div class="drawer-note">${warnings.map((warning) => escapeHtml(warning)).join('<br>')}</div>` : ''}
        <div class="detail-row" style="margin-top:14px;border-top:1px solid rgba(255,255,255,.05)">
          <span class="key">Danger</span>
          <span class="value"><button class="btn small danger" id="drawer-remove">${icon('trash', 13)} Remove from library</button></span>
        </div>
      </div>
    </aside>`;
  $$('[data-close-drawer]', root).forEach((node) => node.addEventListener('click', closeDrawer));
  $('#drawer-play', root).addEventListener('click', () => playGame(game));
  $('#drawer-build', root).addEventListener('click', () => { closeDrawer(); openConvert(game); });
  $('#drawer-folder', root).addEventListener('click', () => {
    const targetPath = game.packagePath || game.gamePath;
    api.shell.openPath(targetPath).catch((error) => toast('Could not open', error.message, 'err'));
  });
  $('#drawer-log', root)?.addEventListener('click', () => {
    api.shell.openPath(`${game.packagePath}/anygame-run.log`).catch((error) => toast('No run log yet', error.message, 'warn'));
  });
  $('#drawer-test', root)?.addEventListener('click', () => { closeDrawer(); runDiagnose(game); });
  $('#drawer-verify', root)?.addEventListener('click', () => { closeDrawer(); runVerify(game); });
  $('#drawer-remove', root).addEventListener('click', () => {
    if (state.settings.confirmRemove === false) removeGame(game);
    else confirmRemove(game);
  });
}

function closeDrawer() {
  $('#drawer-root').innerHTML = '';
  state.selectedId = null;
  $$('.game-card').forEach((card) => card.classList.remove('selected'));
}

/* ------------------------------------------------------------------ */
/* Convert                                                             */
/* ------------------------------------------------------------------ */

function defaultOutput(game) {
  const root = state.settings.outputRoot || state.info.home;
  return `${root.replace(/[\\/]+$/, '')}/${game.titleId || game.id}`;
}

function openConvert(game) {
  const target = game.target || state.settings.target || api.platform;
  const output = game.packagePath || defaultOutput(game);
  const html = `
    <div class="modal-head">
      <div>
        <h3>Convert game</h3>
        <p>${escapeHtml(game.name)} · ${escapeHtml(game.titleId || 'no id')}</p>
      </div>
      <span class="spacer"></span>
      <button class="btn icon ghost" data-modal-close>${icon('x', 16)}</button>
    </div>
    <div class="modal-body">
      <div class="form-grid">
        <div class="field">
          <label>Target platform</label>
          <div class="segmented" id="convert-target">
            <button data-value="linux" class="${target === 'linux' ? 'active' : ''}">Linux</button>
            <button data-value="windows" class="${target === 'windows' ? 'active' : ''}">Windows</button>
          </div>
          <span class="hint">The relinker runs on this host and outputs the selected format.</span>
        </div>
        <div class="field">
          <label>Resources</label>
          <select id="convert-resources">
            <option value="link">Link (hardlinks, fast)</option>
            <option value="copy">Copy (safe, duplicates space)</option>
            <option value="none">None (executable + libs only)</option>
          </select>
        </div>
        <div class="field full">
          <label>Output folder</label>
          <div class="row">
            <input type="text" id="convert-output" value="${escapeAttr(output)}" spellcheck="false">
            <button class="btn" id="convert-browse">${icon('folder', 15)} Browse</button>
          </div>
        </div>
        <div class="field full">
          <label>Patch file (optional)</label>
          <div class="row">
            <input type="text" id="convert-patches" placeholder="relinker-patches.toml" spellcheck="false">
            <button class="btn" id="convert-patch-browse">${icon('folder', 15)} Browse</button>
          </div>
        </div>
        <div class="field">
          <label>Options</label>
          <label class="check"><input type="checkbox" id="convert-skip-modules"> Skip guest modules</label>
          ${game.extraModules && game.extraModules.length ? `<span class="hint">Extra modules: ${escapeHtml(game.extraModules.join(', '))}</span>` : ''}
          <label class="check"><input type="checkbox" id="convert-gui" ${target === 'windows' ? '' : 'disabled'}> Windows GUI subsystem</label>
          <label class="check"><input type="checkbox" id="convert-diagnose" checked> Test after conversion (15 s)</label>
        </div>
        <div class="field">
          <label>&nbsp;</label>
          <label class="check"><input type="checkbox" id="convert-no-patches"> Ignore discovered patches</label>
          <label class="check"><input type="checkbox" id="convert-intel"> Convert AMD-only instructions</label>
        </div>
      </div>
    </div>
    <div class="modal-foot">
      <span class="spacer"></span>
      <button class="btn ghost" data-modal-close>Cancel</button>
      <button class="btn primary" id="convert-start">${icon('spark', 15)} Convert</button>
    </div>`;
  openModal(html);
  $('#convert-resources').value = state.settings.resources || 'link';
  if (game.hasModules === false) $('#convert-skip-modules').checked = true;
  let selectedTarget = target;
  $$('#convert-target button').forEach((button) => {
    button.addEventListener('click', () => {
      selectedTarget = button.dataset.value;
      $$('#convert-target button').forEach((item) => item.classList.toggle('active', item === button));
      $('#convert-gui').disabled = selectedTarget !== 'windows';
    });
  });
  $('#convert-browse').addEventListener('click', async () => {
    const folder = await api.dialog.folder({ title: 'Select the output folder', defaultPath: $('#convert-output').value });
    if (folder) $('#convert-output').value = folder;
  });
  $('#convert-patch-browse').addEventListener('click', async () => {
    const file = await api.dialog.file({ title: 'Select a patch file', filters: [{ name: 'AnyPS5 patches', extensions: ['toml'] }] });
    if (file) $('#convert-patches').value = file;
  });
  $('#convert-start').addEventListener('click', async () => {
    if (!activeRuntimes()[selectedTarget]) {
      closeModal();
      setRoute('runtimes');
      toast('AnyPS5 runtime required', `Install the ${targetLabel(selectedTarget)} runtime first.`, 'warn', 5000);
      return;
    }
    const outputPath = $('#convert-output').value.trim();
    if (!outputPath) { toast('Choose an output folder', '', 'warn'); return; }
    const args = ['build', game.gamePath, '-o', outputPath, '--target', selectedTarget, '--resources', $('#convert-resources').value];
    const patches = $('#convert-patches').value.trim();
    if (patches) args.push('--patches', patches);
    if ($('#convert-no-patches').checked) args.push('--no-patches');
    if ($('#convert-skip-modules').checked) args.push('--skip-modules');
    if ($('#convert-gui').checked && selectedTarget === 'windows') args.push('--windows-gui');
    if ($('#convert-intel').checked) args.push('--to-intel');
    if ($('#convert-diagnose').checked) args.push('--diagnose', '--diagnose-seconds', '15');
    closeModal();
    runJob({
      title: `Converting ${game.name}`,
      subtitle: `${targetLabel(selectedTarget)} · ${outputPath}`,
      args,
      successText: 'Conversion finished',
      onSuccess: async () => {
        await api.library.update({
          id: game.id,
          packagePath: outputPath,
          target: selectedTarget,
          lastBuiltAt: new Date().toISOString(),
          runtime: activeRuntimes()[selectedTarget]?.Version || null,
        });
        await refreshLibrary(true);
      },
    });
  });
}

/* ------------------------------------------------------------------ */
/* Generic job modal                                                   */
/* ------------------------------------------------------------------ */

function runJob({ title, subtitle, args, cwd, onSuccess, successText }) {
  const html = `
    <div class="modal-head">
      <div>
        <h3>${escapeHtml(title)}</h3>
        <p>${escapeHtml(subtitle || '')}</p>
      </div>
      <span class="spacer"></span>
      <span class="chip info" id="job-state"><span class="spinner" style="width:11px;height:11px"></span>&nbsp;Working</span>
    </div>
    <div class="modal-body">
      <pre class="log" id="job-log"></pre>
    </div>
    <div class="modal-foot">
      <span class="spacer"></span>
      <button class="btn ghost" id="job-cancel">Cancel</button>
      <button class="btn primary" id="job-close" style="display:none">Close</button>
    </div>`;
  openModal(html, { wide: true, locked: true });
  const log = $('#job-log');
  const stateChip = $('#job-state');
  const cancel = $('#job-cancel');
  const close = $('#job-close');
  let id = null;
  let finished = false;
  let cancelled = false;
  let buffer = [];

  const append = (type, text) => {
    let cleaned = String(text).replace(/\x1b\[[0-9;?]*[A-Za-z]/g, '').replace(/[^\n]*\r/g, '');
    if (!cleaned) return;
    const span = document.createElement('span');
    if (type === 'stderr') span.className = 'stderr';
    span.textContent = cleaned;
    log.appendChild(span);
    log.scrollTop = log.scrollHeight;
  };

  const handle = (event) => {
    if (!id || event.id !== id) { buffer.push(event); return; }
    if (event.type === 'exit') {
      finished = true;
      if (event.code === 0) {
        stateChip.className = 'chip ok';
        stateChip.innerHTML = `${icon('check', 12)}&nbsp;Done`;
        append('stdout', `\n${successText || 'Finished'}.\n`);
        Promise.resolve(onSuccess && onSuccess()).then(() => {
          cancel.style.display = 'none';
          close.style.display = '';
        }).catch((error) => {
          toast('Post-processing failed', error.message, 'err', 6000);
          cancel.style.display = 'none';
          close.style.display = '';
        });
      } else {
        stateChip.className = cancelled ? 'chip warn' : 'chip err';
        stateChip.innerHTML = cancelled
          ? `${icon('alert', 12)}&nbsp;Cancelled`
          : `${icon('alert', 12)}&nbsp;Failed (${event.code})`;
        if (!cancelled) append('stderr', `\nProcess exited with code ${event.code}.\n`);
        cancel.style.display = 'none';
        close.style.display = '';
      }
      renderSidebarRuntime();
      updateStatusBar();
      return;
    }
    append(event.type, event.data);
  };

  const off = api.cli.onEvent(handle);
  const cleanup = () => { off(); };

  api.cli.start({ args, cwd }).then(({ id: jobId }) => {
    id = jobId;
    for (const event of buffer) handle(event);
    buffer = [];
  }).catch((error) => {
    append('stderr', error.message + '\n');
    stateChip.className = 'chip err';
    stateChip.innerHTML = `${icon('alert', 12)}&nbsp;Failed`;
    cancel.style.display = 'none';
    close.style.display = '';
  });

  cancel.addEventListener('click', () => {
    if (finished || !id) return;
    cancelled = true;
    cancel.disabled = true;
    cancel.textContent = 'Cancelling…';
    api.cli.cancel(id);
  });
  close.addEventListener('click', () => { modalLocked = false; off(); closeModal(); });
  modalCloseHooks.push(cleanup);
}

/* ------------------------------------------------------------------ */
/* Modal plumbing                                                      */
/* ------------------------------------------------------------------ */

let modalCloseHooks = [];
let modalLocked = false;

function openModal(html, { wide = false, locked = false } = {}) {
  const root = $('#modal-root');
  root.innerHTML = `<div class="modal-backdrop"><div class="modal ${wide ? 'wide' : ''}" role="dialog">${html}</div></div>`;
  modalLocked = locked;
  const backdrop = $('.modal-backdrop', root);
  backdrop.addEventListener('click', (event) => {
    if (event.target === backdrop && !modalLocked) closeModal();
  });
  $$('[data-modal-close]', root).forEach((button) => button.addEventListener('click', () => closeModal()));
}

function closeModal(force = false) {
  if (modalLocked && !force) return;
  modalLocked = false;
  const hooks = modalCloseHooks;
  modalCloseHooks = [];
  $('#modal-root').innerHTML = '';
  for (const hook of hooks) { try { hook(); } catch (_) {} }
}

/* ------------------------------------------------------------------ */
/* Actions                                                             */
/* ------------------------------------------------------------------ */

function diagnosisLabel(verdict) {
  switch (verdict) {
    case 'running': return 'Runs (window)';
    case 'exited': return 'Exited cleanly';
    case 'started': return 'Started, no window';
    case 'missing-libraries': return 'Missing libraries';
    case 'crashed': return 'Crashed';
    case 'no-window': return 'No window';
    default: return String(verdict || 'unknown');
  }
}

function runDiagnose(game) {
  if (!game.packagePath) { toast('Convert the game first', '', 'warn'); return; }
  runJob({
    title: `Testing ${game.name}`,
    subtitle: `15 seconds · ${game.packagePath}`,
    args: ['diagnose', game.packagePath, '--seconds', '15'],
    successText: 'Diagnosis finished',
    onSuccess: async () => { await refreshLibrary(true); },
  });
}

function runVerify(game) {
  if (!game.packagePath) { toast('Convert the game first', '', 'warn'); return; }
  runJob({
    title: `Verifying ${game.name}`,
    subtitle: game.packagePath,
    args: ['verify', game.packagePath],
    successText: 'Package is consistent',
    onSuccess: async () => {},
  });
}

async function playGame(game) {
  try {
    await api.game.play(game.id);
    toast('Launching', game.name, 'ok', 2400);
  } catch (error) {
    toast('Could not launch', error.message, 'err', 6000);
  }
}

function confirmRemove(game) {
  const html = `
    <div class="modal-head">
      <div><h3>Remove from library?</h3><p>${escapeHtml(game.name)}</p></div>
      <span class="spacer"></span>
      <button class="btn icon ghost" data-modal-close>${icon('x', 16)}</button>
    </div>
    <div class="modal-body">
      <p style="color:var(--muted);font-size:12.5px;line-height:1.6;margin:0">
        The entry is removed from AnyGame. The game folder and any converted package stay on disk.
      </p>
    </div>
    <div class="modal-foot">
      <span class="spacer"></span>
      <button class="btn ghost" data-modal-close>Cancel</button>
      <button class="btn danger" id="remove-confirm">${icon('trash', 15)} Remove</button>
    </div>`;
  openModal(html);
  $('#remove-confirm').addEventListener('click', () => removeGame(game));
}

async function removeGame(game) {
  try {
    await api.library.remove(game.id);
    closeModal();
    closeDrawer();
    await refreshLibrary(true);
    toast('Removed', game.name, 'ok');
  } catch (error) {
    toast('Could not remove', error.message, 'err');
  }
}

async function checkUpdates() {
  try {
    toast('Checking AnyPS5 releases…', '', 'info', 2000);
    state.updateInfo = await api.runtime.check();
    if (state.route === 'runtimes') renderRuntimes();
    const updates = state.updateInfo.filter((item) => item.updateAvailable);
    if (updates.length) {
      toast('Update available', updates.map((item) => `${item.target}: ${item.latest}`).join(' · '), 'warn', 6000);
    } else {
      toast('Up to date', 'AnyPS5 is already at the latest release.', 'ok');
    }
  } catch (error) {
    toast('Check failed', error.message, 'err', 7000);
  }
}

function updateRuntime(target) {
  const platform = target === 'all' ? 'all' : (target || state.settings.target || api.platform);
  runJob({
    title: 'Installing AnyPS5 runtime',
    subtitle: platform === 'all' ? 'Linux + Windows libraries' : targetLabel(platform),
    args: ['update', '--target', platform],
    successText: 'Runtime installed',
    onSuccess: async () => {
      state.updateInfo = null;
      await refreshRuntimes(true);
      await refreshDoctor();
      if (state.route === 'runtimes') renderRuntimes();
    },
  });
}

/* ------------------------------------------------------------------ */
/* Runtimes page                                                       */
/* ------------------------------------------------------------------ */

function renderRuntimes() {
  if (state.route !== 'runtimes') return;
  const body = $('#page-body');
  const byTarget = activeRuntimes();
  const checkByTarget = {};
  for (const item of state.updateInfo || []) checkByTarget[item.target] = item;

  const targetCard = (target) => {
    const runtime = byTarget[target];
    const check = checkByTarget[target];
    const update = check && check.updateAvailable;
    return `
      <div class="panel ${runtime ? 'accent' : ''}">
        <div class="panel-head">
          <span class="panel-icon">${icon(target === 'windows' ? 'monitor' : 'gamepad', 17)}</span>
          <h3>${targetLabel(target)}</h3>
          <span class="spacer"></span>
          ${runtime ? `<span class="chip ok"><span class="dot"></span>Installed</span>` : `<span class="chip warn"><span class="dot"></span>Missing</span>`}
          ${update ? `<span class="chip info">${escapeHtml(check.latest)} available</span>` : ''}
        </div>
        <div class="kv"><span>Version</span><span>${runtime ? escapeHtml(runtime.Version) : '—'}</span></div>
        <div class="kv"><span>Relinker</span><span>${runtime ? escapeHtml(runtime.Manifest?.relinker?.name || '—') : '—'}</span></div>
        <div class="kv"><span>Libraries</span><span>${runtime ? escapeHtml(runtime.Manifest?.libs?.name || '—') : '—'}</span></div>
        <div style="display:flex;gap:8px;margin-top:12px">
          <button class="btn primary small" data-update="${target}">${icon('download', 13)} ${runtime ? 'Update' : 'Install'}</button>
          ${runtime ? `<button class="btn small" data-reinstall="${target}">${icon('refresh', 13)} Reinstall</button>` : ''}
        </div>
      </div>`;
  };

  const rows = state.runtimes.map((runtime) => {
    const active = (byTarget[runtime.Target]?.Version || '') === runtime.Version;
    return `<tr>
      <td><strong>${escapeHtml(runtime.Version)}</strong></td>
      <td>${escapeHtml(targetLabel(runtime.Target))}</td>
      <td>${escapeHtml(new Date(runtime.Manifest?.installedAt || Date.now()).toLocaleDateString())}</td>
      <td class="mono">${escapeHtml(runtime.Dir || '')}</td>
      <td>${active ? '<span class="chip ok">active</span>' : '<span class="chip">installed</span>'}</td>
    </tr>`;
  }).join('');

  body.innerHTML = `
    <section class="section">
      <div class="section-head">
        <div><h2>Platforms</h2><p>One runtime per target. The relinker binary always matches this host.</p></div>
        <span class="spacer"></span>
        ${state.updateInfo ? '<span class="chip info">release check complete</span>' : ''}
      </div>
      <div class="duo">
        ${targetCard('linux')}
        ${targetCard('windows')}
      </div>
    </section>
    <section class="section">
      <div class="section-head"><div><h2>Installed builds</h2><p>Downloaded from the official AnyPS5 releases.</p></div></div>
      <div class="panel">
        ${state.runtimes.length ? `<table class="table">
          <thead><tr><th>Version</th><th>Target</th><th>Installed</th><th>Directory</th><th>State</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>` : '<p style="padding:8px 0">No runtimes installed yet. Use <strong>Install latest</strong> to download the official release.</p>'}
      </div>
    </section>
    <section class="section">
      <div class="section-head"><div><h2>Host checks</h2><p>What AnyPS5 needs to run converted titles.</p></div></div>
      <div class="panel">
        ${state.doctor.length ? state.doctor.map((check) => `
          <div class="check-row">
            <span class="chip ${check.status === 'ok' ? 'ok' : check.status === 'fail' ? 'err' : 'warn'}">${escapeHtml(check.status)}</span>
            <span class="name">${escapeHtml(check.name)}</span>
            <span class="detail">${escapeHtml(check.detail)}</span>
          </div>`).join('') : '<p>Diagnostics unavailable.</p>'}
      </div>
    </section>`;

  $$('[data-update]', body).forEach((button) => button.addEventListener('click', () => updateRuntime(button.dataset.update)));
  $$('[data-reinstall]', body).forEach((button) => button.addEventListener('click', () => {
    runJob({
      title: 'Reinstalling AnyPS5 runtime',
      subtitle: targetLabel(button.dataset.reinstall),
      args: ['update', '--target', button.dataset.reinstall, '--force'],
      successText: 'Runtime reinstalled',
      onSuccess: async () => { await refreshRuntimes(true); if (state.route === 'runtimes') renderRuntimes(); },
    });
  }));
}

/* ------------------------------------------------------------------ */
/* Settings page                                                       */
/* ------------------------------------------------------------------ */

function renderSettings() {
  if (state.route !== 'settings') return;
  const body = $('#page-body');
  const settings = state.settings;
  const active = activeRuntimes();
  body.innerHTML = `
    <div class="settings-grid">
      <div>
        <div class="settings-card">
          <div class="head"><h3>Conversion defaults</h3><p>Applied to new conversions. Every game can override them.</p></div>
          <div class="body">
            ${settingRow('Default target', 'Format produced by “Convert” unless overridden.', `
              <select class="select" data-setting="target">
                <option value="linux" ${settings.target === 'linux' ? 'selected' : ''}>Linux (ELF)</option>
                <option value="windows" ${settings.target === 'windows' ? 'selected' : ''}>Windows (PE)</option>
              </select>`)}
            ${settingRow('Resources', 'How app0/ receives the title files.', `
              <select class="select" data-setting="resources">
                <option value="link" ${settings.resources === 'link' ? 'selected' : ''}>Hardlink</option>
                <option value="copy" ${settings.resources === 'copy' ? 'selected' : ''}>Copy</option>
                <option value="none" ${settings.resources === 'none' ? 'selected' : ''}>None</option>
              </select>`)}
            ${settingRow('Output root', 'Parent folder for new packages.', `
              <input type="text" data-setting="outputRoot" value="${escapeAttr(settings.outputRoot || '')}" spellcheck="false">`)}
            ${settingRow('Confirm remove', 'Ask before removing an entry from the library.', `
              <label class="switch"><input type="checkbox" data-setting="confirmRemove" ${settings.confirmRemove ? 'checked' : ''}><span></span></label>`)}
          </div>
        </div>
        <div class="settings-card">
          <div class="head"><h3>Environment</h3><p>Passed to conversions and launches.</p></div>
          <div class="body">
            ${settingRow('Data directory', 'Where runtimes and library.json live.', `
              <input type="text" data-setting="home" value="${escapeAttr(settings.home || state.info.home)}" spellcheck="false">`)}
            ${settingRow('GPU filter', 'Optional ANYPS5_GPU substring for launches.', `
              <input type="text" data-setting="gpu" placeholder="e.g. RTX" value="${escapeAttr(settings.gpu || '')}" spellcheck="false">`)}
            ${settingRow('Fullscreen', 'Start converted games fullscreen.', `
              <label class="switch"><input type="checkbox" data-setting="fullscreen" ${settings.fullscreen ? 'checked' : ''}><span></span></label>`)}
          </div>
        </div>
      </div>
      <div>
        <div class="settings-card">
          <div class="head"><h3>Runtime</h3><p>Active AnyPS5 builds.</p></div>
          <div class="body">
            <div class="kv" style="display:flex;justify-content:space-between;padding:8px 0;border-bottom:1px solid rgba(255,255,255,.05)"><span style="color:var(--faint)">Linux</span><span class="mono">${active.linux ? escapeHtml(active.linux.Version) : 'not installed'}</span></div>
            <div class="kv" style="display:flex;justify-content:space-between;padding:8px 0"><span style="color:var(--faint)">Windows</span><span class="mono">${active.windows ? escapeHtml(active.windows.Version) : 'not installed'}</span></div>
            <button class="btn small" id="settings-runtimes" style="margin-top:10px">${icon('cpu', 13)} Manage runtimes</button>
          </div>
        </div>
        <div class="settings-card">
          <div class="head"><h3>Diagnostics</h3><p>Host readiness for converted titles.</p></div>
          <div class="body">
            ${state.doctor.length ? state.doctor.map((check) => `
              <div class="check-row">
                <span class="chip ${check.status === 'ok' ? 'ok' : check.status === 'fail' ? 'err' : 'warn'}">${escapeHtml(check.status)}</span>
                <span class="name">${escapeHtml(check.name)}</span>
              </div>`).join('') : '<p>No diagnostics yet.</p>'}
            <button class="btn small" id="settings-doctor" style="margin-top:10px">${icon('refresh', 13)} Run checks</button>
          </div>
        </div>
      </div>
    </div>`;
  $('#settings-runtimes')?.addEventListener('click', () => setRoute('runtimes'));
  $('#settings-doctor')?.addEventListener('click', async () => { await refreshDoctor(); renderSettings(); });
  wireSettingsInputs(body);
}

function settingRow(name, description, control) {
  return `<div class="setting-row">
    <div class="copy"><div class="name">${escapeHtml(name)}</div><div class="desc">${escapeHtml(description)}</div></div>
    <div class="control">${control}</div>
  </div>`;
}

function wireSettingsInputs(root) {
  $$('[data-setting]', root).forEach((input) => {
    const key = input.dataset.setting;
    const commit = async () => {
      let value;
      if (input.type === 'checkbox') value = input.checked;
      else value = input.value.trim();
      try {
        state.settings = await api.settings.save({ [key]: value });
        state.info.settings = state.settings;
        if (key === 'home') {
          state.info = await api.appInfo();
          await refreshLibrary(true);
          await refreshRuntimes(true);
        }
        updateStatusBar();
        toast('Settings saved', '', 'ok', 1600);
      } catch (error) {
        toast('Could not save settings', error.message, 'err');
      }
    };
    if (input.type === 'checkbox' || input.tagName === 'SELECT') input.addEventListener('change', commit);
    else input.addEventListener('blur', commit);
  });
}

/* ------------------------------------------------------------------ */
/* About page                                                          */
/* ------------------------------------------------------------------ */

function renderAbout() {
  if (state.route !== 'about') return;
  const body = $('#page-body');
  const active = activeRuntimes();
  const anyps5 = active.linux?.Version || active.windows?.Version || 'not installed';
  body.innerHTML = `
    <div class="about-hero">
      <img class="about-logo" src="../assets/icon.png" alt="AnyGame">
      <div>
        <h1>AnyGame</h1>
        <p>A desktop library and conversion launcher for PS5 titles, built on top of the official AnyPS5 relinker and system libraries. AnyGame never ships game data, firmware or keys.</p>
        <div style="display:flex;gap:10px;margin-top:14px">
          <button class="btn small" id="about-anyps5">${icon('external', 13)} AnyPS5 on GitHub</button>
          <button class="btn small" id="about-docs">${icon('external', 13)} Usage guide</button>
        </div>
      </div>
    </div>
    <div class="version-grid">
      <div class="version-cell"><div class="k">AnyGame</div><div class="v">${escapeHtml(state.info.version)}</div></div>
      <div class="version-cell"><div class="k">AnyPS5</div><div class="v">${escapeHtml(anyps5)}</div></div>
      <div class="version-cell"><div class="k">Electron</div><div class="v">${escapeHtml(state.info.electron)}</div></div>
      <div class="version-cell"><div class="k">Chromium</div><div class="v">${escapeHtml(state.info.chrome)}</div></div>
    </div>
    <div class="settings-card" style="margin-top:16px">
      <div class="head"><h3>Paths</h3><p>Everything is local.</p></div>
      <div class="body">
        <div class="detail-row"><span class="key">CLI</span><span class="value mono">${escapeHtml(state.info.cli)}</span></div>
        <div class="detail-row"><span class="key">Data</span><span class="value mono">${escapeHtml(state.info.home)}</span></div>
        <div class="detail-row"><span class="key">Settings</span><span class="value mono">${escapeHtml(state.info.configFile)}</span></div>
        <div class="detail-row"><span class="key">Library</span><span class="value mono">${escapeHtml(state.info.libraryFile)}</span></div>
      </div>
    </div>
    <p style="color:var(--faint);font-size:11px;line-height:1.6;margin-top:18px">
      AnyGame is licensed under GPL-2.0. AnyPS5 is an independent project, not affiliated with Sony Interactive Entertainment.
      “PlayStation” and “PS5” are trademarks of Sony Interactive Entertainment Inc. Use only with content you legally own.
    </p>`;
  $('#about-anyps5').addEventListener('click', () => api.shell.openExternal('https://github.com/boykopovar/AnyPS5'));
  $('#about-docs').addEventListener('click', () => api.shell.openExternal('https://github.com/boykopovar/AnyPS5/blob/main/docs/user/USAGE.md'));
}
