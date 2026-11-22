// SPDX-License-Identifier: GPL-2.0-only

const { safeStorage } = require('electron');
const fs = require('fs');
const path = require('path');

const DEVICE_CODE_URL = 'https://github.com/login/device/code';
const TOKEN_URL = 'https://github.com/login/oauth/access_token';
const API_URL = 'https://api.github.com';
const SCOPE = 'public_repo';

module.exports = function createGitHub({ app, getClientId, send }) {
  const file = path.join(app.getPath('userData'), 'github-auth.json');
  let pending = null;
  let timer = null;

  const notify = (payload) => {
    if (typeof send === 'function') send(payload);
  };

  function load() {
    try {
      const stored = JSON.parse(fs.readFileSync(file, 'utf8'));
      if (stored.encrypted) {
        if (!safeStorage.isEncryptionAvailable()) return null;
        return JSON.parse(safeStorage.decryptString(Buffer.from(stored.data, 'base64')));
      }
      return JSON.parse(stored.data);
    } catch (_) {
      return null;
    }
  }

  function save(token, user) {
    const payload = JSON.stringify({ token, user });
    const stored = safeStorage.isEncryptionAvailable()
      ? { v: 1, encrypted: true, data: safeStorage.encryptString(payload).toString('base64') }
      : { v: 1, encrypted: false, data: payload };
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, JSON.stringify(stored), { mode: 0o600 });
  }

  function clear() {
    try { fs.unlinkSync(file); } catch (_) {}
  }

  function status() {
    const stored = load();
    return {
      signedIn: Boolean(stored && stored.token),
      user: stored ? stored.user : null,
      clientIdConfigured: Boolean(getClientId()),
      encrypted: safeStorage.isEncryptionAvailable(),
    };
  }

  async function getUser(token) {
    const response = await fetch(`${API_URL}/user`, {
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: 'application/vnd.github+json',
        'User-Agent': 'AnyGame',
      },
    });
    if (!response.ok) {
      if (response.status === 401) throw new Error('GitHub rejected the token');
      throw new Error(`GitHub API returned ${response.status}`);
    }
    const user = await response.json();
    return {
      login: user.login,
      name: user.name || user.login,
      avatar: user.avatar_url,
      scopes: response.headers.get('x-oauth-scopes') || '',
    };
  }

  function stopPolling() {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function cancel() {
    if (pending) pending.cancelled = true;
    pending = null;
    stopPolling();
    notify({ status: 'cancelled' });
    return { ok: true };
  }

  function fail(message) {
    pending = null;
    stopPolling();
    notify({ status: 'error', error: message });
  }

  function schedule() {
    if (!pending || pending.cancelled) return;
    stopPolling();
    timer = setTimeout(poll, pending.interval * 1000);
    timer.unref?.();
  }

  async function poll() {
    if (!pending || pending.cancelled) return;
    if (Date.now() >= pending.expiresAt) {
      fail('The GitHub code expired. Start the login again.');
      return;
    }
    try {
      const response = await fetch(TOKEN_URL, {
        method: 'POST',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/x-www-form-urlencoded',
        },
        body: new URLSearchParams({
          client_id: getClientId(),
          device_code: pending.deviceCode,
          grant_type: 'urn:ietf:params:oauth:grant-type:device_code',
        }),
      });
      const data = await response.json();
      if (data.access_token) {
        const user = await getUser(data.access_token);
        save(data.access_token, user);
        pending = null;
        stopPolling();
        notify({ status: 'signed-in', user });
        return;
      }
      if (data.error === 'authorization_pending') {
        schedule();
        return;
      }
      if (data.error === 'slow_down') {
        pending.interval += 5;
        schedule();
        return;
      }
      fail(data.error_description || data.error || 'GitHub did not authorize the login');
    } catch (error) {
      fail(error.message);
    }
  }

  async function start() {
    if (!getClientId()) {
      const error = new Error('No GitHub OAuth client ID configured');
      error.code = 'no-client-id';
      throw error;
    }
    cancel();
    const response = await fetch(DEVICE_CODE_URL, {
      method: 'POST',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      body: new URLSearchParams({ client_id: getClientId(), scope: SCOPE }),
    });
    const data = await response.json();
    if (!response.ok || data.error) {
      const message = data.error === 'device_flow_disabled'
        ? 'Device Flow is not enabled for this GitHub OAuth App.'
        : data.error_description || data.error || `GitHub returned ${response.status}`;
      throw new Error(message);
    }
    pending = {
      deviceCode: data.device_code,
      interval: Math.max(5, Number(data.interval) || 5),
      expiresAt: Date.now() + (Number(data.expires_in) || 900) * 1000,
      cancelled: false,
    };
    schedule();
    notify({
      status: 'pending',
      userCode: data.user_code,
      verificationUri: data.verification_uri,
      expiresIn: data.expires_in,
    });
    return {
      userCode: data.user_code,
      verificationUri: data.verification_uri,
      expiresIn: data.expires_in,
    };
  }

  async function pat(token) {
    const value = String(token || '').trim();
    if (!value) throw new Error('empty token');
    const user = await getUser(value);
    save(value, user);
    notify({ status: 'signed-in', user });
    return { user };
  }

  async function logout() {
    cancel();
    clear();
    notify({ status: 'signed-out' });
    return { ok: true };
  }

  return { status, start, cancel, pat, logout };
};
