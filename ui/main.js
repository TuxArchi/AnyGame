// SPDX-License-Identifier: GPL-2.0-only

const { app, BrowserWindow, ipcMain, dialog, shell, nativeTheme } = require('electron');
const fs = require('fs');
const fsp = fs.promises;
const path = require('path');
const os = require('os');
const { spawn } = require('child_process');
const { pathToFileURL } = require('url');

const packageJson = require('./package.json');
const isWindows = process.platform === 'win32';

app.setName('AnyGame');
if (process.platform === 'linux') app.setDesktopName('anygame.desktop');

let mainWindow = null;
let jobs = new Map();
let jobCounter = 0;

function cliName() {
  return isWindows ? 'anygame.exe' : 'anygame';
}

function resolveCli() {
  if (process.env.ANYGAME_CLI) return process.env.ANYGAME_CLI;
  const candidates = [
    path.join(process.resourcesPath || '', 'bin', cliName()),
    path.join(__dirname, '..', 'bin', cliName()),
  ];
  for (const candidate of candidates) {
    try {
      if (fs.existsSync(candidate)) return candidate;
    } catch (_) {}
  }
  return cliName();
}

function defaultHome() {
  if (process.env.ANYGAME_HOME) return process.env.ANYGAME_HOME;
  if (isWindows) {
    return path.join(process.env.LOCALAPPDATA || path.join(os.homedir(), 'AppData', 'Local'), 'AnyGame');
  }
  return path.join(process.env.XDG_DATA_HOME || path.join(os.homedir(), '.local', 'share'), 'AnyGame');
}

const CLI = resolveCli();
const DEFAULT_HOME = defaultHome();
const CONFIG_FILE = path.join(app.getPath('userData'), 'settings.json');

const DEFAULT_SETTINGS = {
  home: DEFAULT_HOME,
  target: isWindows ? 'windows' : 'linux',
  resources: 'link',
  outputRoot: path.join(os.homedir(), 'AnyGameGames'),
  gpu: '',
  fullscreen: false,
  confirmRemove: true,
};

function readJsonSync(file, fallback) {
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch (_) {
    return fallback;
  }
}

async function readJson(file, fallback) {
  try {
    return JSON.parse(await fsp.readFile(file, 'utf8'));
  } catch (_) {
    return fallback;
  }
}

async function writeJson(file, value) {
  await fsp.mkdir(path.dirname(file), { recursive: true });
  const temporary = file + '.tmp-' + process.pid;
  await fsp.writeFile(temporary, JSON.stringify(value, null, 2) + '\n');
  await fsp.rename(temporary, file);
}

function settings() {
  return { ...DEFAULT_SETTINGS, ...readJsonSync(CONFIG_FILE, {}) };
}

async function saveSettings(patch) {
  const next = { ...settings(), ...patch };
  await writeJson(CONFIG_FILE, next);
  return next;
}

function home() {
  const configured = settings().home;
  return configured && configured.trim() ? configured : DEFAULT_HOME;
}

function libraryFile() {
  return path.join(home(), 'library.json');
}

function cliEnv(extra = {}) {
  const current = settings();
  const env = { ...process.env, ANYGAME_HOME: home(), ...extra };
  if (current.gpu) env.ANYPS5_GPU = current.gpu;
  return env;
}

function runCli(args, options = {}) {
  return new Promise((resolve) => {
    const child = spawn(CLI, args, {
      cwd: options.cwd || undefined,
      env: cliEnv(options.env),
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stdout = '';
    let stderr = '';
    let timer = null;
    if (options.timeout) {
      timer = setTimeout(() => {
        try { child.kill('SIGKILL'); } catch (_) {}
      }, options.timeout);
    }
    child.stdout.on('data', (chunk) => { stdout += chunk.toString(); });
    child.stderr.on('data', (chunk) => { stderr += chunk.toString(); });
    child.on('error', (error) => {
      if (timer) clearTimeout(timer);
      resolve({ code: -1, stdout, stderr: stderr + error.message });
    });
    child.on('close', (code) => {
      if (timer) clearTimeout(timer);
      resolve({ code, stdout, stderr });
    });
  });
}

async function library() {
  const data = await readJson(libraryFile(), { format: 'anygame.ui-library/1', games: [] });
  if (!Array.isArray(data.games)) data.games = [];
  return data;
}

async function saveLibrary(data) {
  data.format = 'anygame.ui-library/1';
  await writeJson(libraryFile(), data);
}

function executableIn(packagePath) {
  const manifest = readJsonSync(path.join(packagePath, 'anygame.json'), null);
  if (manifest && manifest.executable) {
    const candidate = path.join(packagePath, manifest.executable);
    return { manifest, executable: candidate, exists: fs.existsSync(candidate) };
  }
  try {
    for (const name of fs.readdirSync(packagePath)) {
      if (name.toLowerCase().endsWith('.elf') || name.toLowerCase().endsWith('.exe')) {
        const candidate = path.join(packagePath, name);
        return { manifest, executable: candidate, exists: true };
      }
    }
  } catch (_) {}
  return { manifest, executable: null, exists: false };
}

function decorate(game) {
  const decorated = {
    ...game,
    runtimeVersion: game.runtimeVersion || game.runtime || null,
    iconUrl: game.icon && fs.existsSync(game.icon) ? pathToFileURL(game.icon).href : null,
    status: 'not-converted',
    packageTarget: null,
    executable: null,
    manifest: null,
  };
  if (game.packagePath) {
    const info = executableIn(game.packagePath);
    decorated.packageTarget = info.manifest ? info.manifest.target : game.target || null;
    decorated.executable = info.executable;
    decorated.manifest = info.manifest;
    decorated.status = info.exists ? 'ready' : 'missing';
    if (info.manifest && info.manifest.runtime) {
      decorated.runtimeVersion = info.manifest.runtime.version || decorated.runtimeVersion;
    }
  }
  return decorated;
}

async function listLibrary() {
  const data = await library();
  return { ...data, games: data.games.map(decorate) };
}

function send(channel, payload) {
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.webContents.send(channel, payload);
  }
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1320,
    height: 860,
    minWidth: 1040,
    minHeight: 680,
    show: false,
    frame: false,
    backgroundColor: '#0a0c11',
    title: 'AnyGame',
    icon: path.join(__dirname, 'assets', 'icon.png'),
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      spellcheck: false,
    },
  });
  mainWindow.setMenuBarVisibility(false);
  mainWindow.loadFile(path.join(__dirname, 'renderer', 'index.html'));
  mainWindow.once('ready-to-show', () => mainWindow.show());
  mainWindow.on('maximize', () => send('window:state', { maximized: true }));
  mainWindow.on('unmaximize', () => send('window:state', { maximized: false }));
  mainWindow.webContents.on('console-message', (_event, level, message) => {
    if (level >= 2) console.log('[renderer]', message);
  });
  mainWindow.webContents.on('did-fail-load', (_event, code, description, url) => {
    console.error('[renderer] load failed', code, description, url);
  });
  mainWindow.on('closed', () => { mainWindow = null; });
}

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.focus();
    }
  });
  app.whenReady().then(() => {
    nativeTheme.themeSource = 'dark';
    createWindow();
    app.on('activate', () => {
      if (BrowserWindow.getAllWindows().length === 0) createWindow();
    });
  });
  app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') app.quit();
  });
}

ipcMain.handle('app:info', async () => {
  const current = settings();
  return {
    version: packageJson.version,
    electron: process.versions.electron,
    chrome: process.versions.chrome,
    node: process.versions.node,
    platform: process.platform,
    arch: process.arch,
    cli: CLI,
    home: home(),
    configFile: CONFIG_FILE,
    libraryFile: libraryFile(),
    settings: current,
    defaults: DEFAULT_SETTINGS,
  };
});

ipcMain.handle('settings:save', async (_event, patch) => {
  if (!patch || typeof patch !== 'object') throw new Error('invalid settings');
  const allowed = ['home', 'target', 'resources', 'outputRoot', 'gpu', 'fullscreen', 'confirmRemove'];
  const clean = {};
  for (const key of allowed) if (key in patch) clean[key] = patch[key];
  return saveSettings(clean);
});

ipcMain.handle('library:list', listLibrary);

ipcMain.handle('library:add', async (_event, payload) => {
  let target = payload && payload.path;
  if (!target) {
    const result = await dialog.showOpenDialog(mainWindow, {
      title: 'Select an extracted PS5 game folder',
      properties: ['openDirectory'],
    });
    if (result.canceled || result.filePaths.length === 0) return null;
    target = result.filePaths[0];
  }
  const info = await runCli(['info', target, '--json'], { timeout: 120000 });
  if (info.code !== 0 || !info.stdout.trim()) {
    throw new Error((info.stderr || info.stdout || 'could not read the title').trim());
  }
  let parsed;
  try {
    parsed = JSON.parse(info.stdout);
  } catch (error) {
    throw new Error('anygame info returned invalid JSON: ' + error.message);
  }
  const data = await library();
  const titleId = parsed.TitleID || path.basename(parsed.Resources || target);
  const existing = data.games.find((game) => game.gamePath === parsed.Resources);
  const byId = data.games.find((game) => game.titleId === titleId);
  const game = existing || byId || { id: titleId + '-' + Date.now().toString(36) };
  Object.assign(game, {
    titleId,
    name: parsed.TitleName || titleId,
    gamePath: parsed.Resources || target,
    icon: parsed.Icon || null,
    kind: parsed.Kind,
    contentVersion: parsed.ContentVersion || '',
    hasModules: (Array.isArray(parsed.ModuleDirs) && parsed.ModuleDirs.length > 0) || (Array.isArray(parsed.ExtraModuleDirs) && parsed.ExtraModuleDirs.length > 0),
    extraModules: parsed.ExtraModuleDirs || [],
    eboot: parsed.Eboot,
    warnings: parsed.Warnings || [],
    addedAt: game.addedAt || new Date().toISOString(),
  });
  if (!game.packagePath) {
    game.target = settings().target;
  }
  if (!existing) data.games.push(game);
  await saveLibrary(data);
  return decorate(game);
});

ipcMain.handle('library:update', async (_event, patch) => {
  if (!patch || typeof patch !== 'object' || !patch.id) throw new Error('invalid library update');
  const data = await library();
  const game = data.games.find((item) => item.id === patch.id);
  if (!game) throw new Error('game not found');
  const allowed = ['name', 'packagePath', 'target', 'lastBuiltAt', 'runtime', 'notes'];
  for (const key of allowed) if (key in patch) game[key] = patch[key];
  await saveLibrary(data);
  return decorate(game);
});

ipcMain.handle('library:remove', async (_event, id) => {
  const data = await library();
  data.games = data.games.filter((game) => game.id !== id);
  await saveLibrary(data);
  return { ok: true };
});

ipcMain.handle('dialog:folder', async (_event, options = {}) => {
  const result = await dialog.showOpenDialog(mainWindow, {
    title: options.title || 'Select a folder',
    defaultPath: options.defaultPath || undefined,
    properties: ['openDirectory', 'createDirectory'],
  });
  return result.canceled || result.filePaths.length === 0 ? null : result.filePaths[0];
});

ipcMain.handle('dialog:file', async (_event, options = {}) => {
  const result = await dialog.showOpenDialog(mainWindow, {
    title: options.title || 'Select a file',
    defaultPath: options.defaultPath || undefined,
    properties: ['openFile'],
    filters: options.filters || [{ name: 'All files', extensions: ['*'] }],
  });
  return result.canceled || result.filePaths.length === 0 ? null : result.filePaths[0];
});

ipcMain.handle('cli:run', async (_event, payload = {}) => {
  if (!Array.isArray(payload.args)) throw new Error('invalid arguments');
  return runCli(payload.args, { cwd: payload.cwd, timeout: payload.timeout });
});

ipcMain.handle('cli:start', async (_event, payload = {}) => {
  if (!Array.isArray(payload.args)) throw new Error('invalid arguments');
  const id = String(++jobCounter);
  const child = spawn(CLI, payload.args, {
    cwd: payload.cwd || undefined,
    env: cliEnv(payload.env),
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  jobs.set(id, child);
  const forward = (type, data) => send('cli:event', { id, type, data: data.toString() });
  child.stdout.on('data', (chunk) => forward('stdout', chunk));
  child.stderr.on('data', (chunk) => forward('stderr', chunk));
  child.on('error', (error) => {
    forward('stderr', error.message + '\n');
    jobs.delete(id);
    send('cli:event', { id, type: 'exit', code: -1 });
  });
  child.on('close', (code) => {
    jobs.delete(id);
    send('cli:event', { id, type: 'exit', code });
  });
  return { id };
});

ipcMain.handle('cli:cancel', async (_event, id) => {
  const child = jobs.get(String(id));
  if (!child) return false;
  try { child.kill('SIGTERM'); } catch (_) {}
  setTimeout(() => {
    if (jobs.has(String(id))) {
      try { child.kill('SIGKILL'); } catch (_) {}
    }
  }, 4000).unref?.();
  return true;
});

ipcMain.handle('runtime:runtimes', async () => {
  const result = await runCli(['runtimes', '--json'], { timeout: 60000 });
  if (result.code !== 0) throw new Error((result.stderr || result.stdout).trim());
  try { return JSON.parse(result.stdout); } catch (_) { return []; }
});

ipcMain.handle('runtime:check', async () => {
  const result = await runCli(['update', '--check', '--target', 'all', '--json'], { timeout: 120000 });
  if (result.code !== 0) throw new Error((result.stderr || result.stdout).trim());
  try { return JSON.parse(result.stdout); } catch (_) { return []; }
});

ipcMain.handle('doctor', async () => {
  const result = await runCli(['doctor', '--json'], { timeout: 60000 });
  try { return JSON.parse(result.stdout); } catch (_) { return []; }
});

ipcMain.handle('game:play', async (_event, id) => {
  const data = await library();
  const game = data.games.find((item) => item.id === id);
  if (!game) throw new Error('game not found');
  if (!game.packagePath) throw new Error('convert the game first');
  const info = executableIn(game.packagePath);
  if (!info.exists) throw new Error('the converted executable is missing');
  if (info.manifest && info.manifest.target && info.manifest.target !== process.platform) {
    throw new Error(`this package targets ${info.manifest.target}; run it on ${info.manifest.target}`);
  }
  const current = settings();
  const env = cliEnv();
  if (current.fullscreen) env.ANYPS5_FULLSCREEN = '1';
  const logPath = path.join(game.packagePath, 'anygame-run.log');
  let logFd = null;
  try {
    logFd = fs.openSync(logPath, 'a');
    fs.writeSync(logFd, `\n=== AnyGame run ${new Date().toISOString()} ===\n`);
  } catch (_) {
    logFd = null;
  }
  const child = spawn(CLI, ['run', game.packagePath], {
    cwd: game.packagePath,
    env,
    detached: true,
    stdio: ['ignore', logFd ?? 'ignore', logFd ?? 'ignore'],
    windowsHide: false,
  });
  child.unref();
  if (logFd !== null) {
    try { fs.closeSync(logFd); } catch (_) {}
  }
  return { ok: true, log: logPath };
});

ipcMain.handle('shell:openPath', async (_event, target) => {
  if (!target || typeof target !== 'string') throw new Error('invalid path');
  const result = await shell.openPath(target);
  if (result) throw new Error(result);
  return { ok: true };
});

ipcMain.handle('shell:showItem', async (_event, target) => {
  if (!target || typeof target !== 'string') throw new Error('invalid path');
  shell.showItemInFolder(target);
  return { ok: true };
});

ipcMain.handle('shell:openExternal', async (_event, url) => {
  if (!/^https:\/\//.test(String(url))) throw new Error('only https links are allowed');
  await shell.openExternal(url);
  return { ok: true };
});

ipcMain.on('window:minimize', () => mainWindow?.minimize());
ipcMain.on('window:toggle', () => {
  if (!mainWindow) return;
  if (mainWindow.isMaximized()) mainWindow.unmaximize();
  else mainWindow.maximize();
});
ipcMain.on('window:close', () => mainWindow?.close());
