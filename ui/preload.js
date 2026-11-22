// SPDX-License-Identifier: GPL-2.0-only

const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('anygame', {
  platform: process.platform,
  appInfo: () => ipcRenderer.invoke('app:info'),
  settings: {
    save: (patch) => ipcRenderer.invoke('settings:save', patch),
  },
  library: {
    list: () => ipcRenderer.invoke('library:list'),
    add: (payload) => ipcRenderer.invoke('library:add', payload),
    update: (patch) => ipcRenderer.invoke('library:update', patch),
    remove: (id) => ipcRenderer.invoke('library:remove', id),
  },
  dialog: {
    folder: (options) => ipcRenderer.invoke('dialog:folder', options),
    file: (options) => ipcRenderer.invoke('dialog:file', options),
  },
  cli: {
    run: (payload) => ipcRenderer.invoke('cli:run', payload),
    start: (payload) => ipcRenderer.invoke('cli:start', payload),
    cancel: (id) => ipcRenderer.invoke('cli:cancel', id),
    onEvent: (callback) => {
      const handler = (_event, message) => callback(message);
      ipcRenderer.on('cli:event', handler);
      return () => ipcRenderer.removeListener('cli:event', handler);
    },
  },
  runtime: {
    list: () => ipcRenderer.invoke('runtime:runtimes'),
    check: () => ipcRenderer.invoke('runtime:check'),
  },
  doctor: () => ipcRenderer.invoke('doctor'),
  game: {
    play: (id) => ipcRenderer.invoke('game:play', id),
  },
  shell: {
    openPath: (target) => ipcRenderer.invoke('shell:openPath', target),
    showItem: (target) => ipcRenderer.invoke('shell:showItem', target),
    openExternal: (url) => ipcRenderer.invoke('shell:openExternal', url),
  },
  window: {
    minimize: () => ipcRenderer.send('window:minimize'),
    toggle: () => ipcRenderer.send('window:toggle'),
    close: () => ipcRenderer.send('window:close'),
    onState: (callback) => {
      const handler = (_event, message) => callback(message);
      ipcRenderer.on('window:state', handler);
      return () => ipcRenderer.removeListener('window:state', handler);
    },
  },
});
