import { BaoDemo } from './shim.js';

const logBuffer = [];
const LOG_BUFFER_MAX = 1500;
const originalConsole = {
  log: console.log.bind(console),
  info: console.info.bind(console),
  warn: console.warn.bind(console),
  error: console.error.bind(console),
};

function stringifyConsoleArg(v) {
  if (typeof v === 'string') return v;
  if (v instanceof Error) return `${v.name}: ${v.message}\n${v.stack || ''}`;
  try {
    return JSON.stringify(v);
  } catch (_) {
    return String(v);
  }
}

function pushLogLine(line) {
  logBuffer.push(line);
  if (logBuffer.length > LOG_BUFFER_MAX) {
    logBuffer.splice(0, logBuffer.length - LOG_BUFFER_MAX);
  }
}

function installConsoleCapture() {
  const levels = ['log', 'info', 'warn', 'error'];
  for (const level of levels) {
    console[level] = (...args) => {
      try {
        const text = args.map(stringifyConsoleArg).join(' ');
        pushLogLine(text);
      } catch (_) {}
      originalConsole[level](...args);
    };
  }
}

installConsoleCapture();

function log(msg) {
  originalConsole.log(String(msg ?? ''));
}
const PRIVATE_ID_KEY = 'bao.wasm.privateId';
const CREATOR_PUBLIC_ID_KEY = 'bao.wasm.creatorPublicId';
const STORAGE_CONFIG_KEY = 'bao.wasm.storageConfig';
const AUTO_OPEN_KEY = 'bao.wasm.autoOpen';
const REPLICA_DDL_KEY = 'bao.wasm.replicaDdl';
const WEBAUTHN_ENABLED_KEY = 'bao.wasm.webAuthn.enabled';
const WEBAUTHN_CRED_ID_KEY = 'bao.wasm.webAuthn.credentialId';
const PRIVATE_ID_SEALED_KEY = 'bao.wasm.privateIdSealed';
const SECURE_DB_NAME = 'bao-wasm-secure';
const SECURE_DB_VERSION = 1;
const SECURE_STORE_NAME = 'keys';
const PRIVATE_ID_KEY_HANDLE = 'private-id-key-v1';
let yamlApi = null;

function formatError(e) {
  if (!e) return 'unknown error';
  if (typeof e === 'string') return e;
  const parts = [];
  if (e.name) parts.push(`name=${e.name}`);
  if (e.message) parts.push(`message=${e.message}`);
  if (e.stack) parts.push(`stack=${e.stack}`);
  try {
    parts.push(`raw=${JSON.stringify(e)}`);
  } catch (_) {
    parts.push(`raw=${String(e)}`);
  }
  return parts.join('\n');
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function normalizeDir(d) {
  return (d || '').trim().replace(/^\/+|\/+$/g, '');
}

function joinDir(base, child) {
  const b = normalizeDir(base);
  const c = normalizeDir(child);
  if (!b) return c;
  if (!c) return b;
  return `${b}/${c}`;
}

function parentDir(d) {
  const n = normalizeDir(d);
  if (!n) return '';
  const i = n.lastIndexOf('/');
  return i <= 0 ? '' : n.slice(0, i);
}

function formatModTime(v) {
  try {
    const d = new Date(v);
    if (!Number.isNaN(d.getTime())) return d.toLocaleString();
  } catch (_) {}
  return String(v ?? '');
}

function formatSize(size) {
  const n = Number(size || 0);
  if (!Number.isFinite(n) || n <= 0) return '0 B';
  const mb = n / (1024 * 1024);
  if (mb >= 0.01) {
    return `${mb.toFixed(2)} MB`;
  }
  const kb = n / 1024;
  if (kb >= 0.1) {
    return `${kb.toFixed(1)} KB`;
  }
  return `${Math.round(n)} B`;
}

function decodeBase64ToBytes(base64) {
  const binary = atob(base64 || '');
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function bytesToBase64(bytes) {
  const chunk = 0x8000;
  let binary = '';
  for (let i = 0; i < bytes.length; i += chunk) {
    const sub = bytes.subarray(i, i + chunk);
    binary += String.fromCharCode.apply(null, sub);
  }
  return btoa(binary);
}

function bytesToBase64Url(bytes) {
  return bytesToBase64(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

function base64UrlToBytes(base64url) {
  const s = String(base64url || '').replace(/-/g, '+').replace(/_/g, '/');
  const padded = s + '='.repeat((4 - (s.length % 4 || 4)) % 4);
  return decodeBase64ToBytes(padded);
}

function randomBytes(size) {
  const out = new Uint8Array(size);
  crypto.getRandomValues(out);
  return out;
}

function hasWebAuthn() {
  return !!(globalThis.PublicKeyCredential && navigator.credentials && navigator.credentials.create && navigator.credentials.get);
}

function openSecureDb() {
  return new Promise((resolve, reject) => {
    if (!globalThis.indexedDB) {
      reject(new Error('indexedDB is unavailable'));
      return;
    }
    const req = indexedDB.open(SECURE_DB_NAME, SECURE_DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(SECURE_STORE_NAME)) {
        db.createObjectStore(SECURE_STORE_NAME, { keyPath: 'id' });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error || new Error('cannot open secure db'));
  });
}

async function secureStorePut(id, value) {
  const db = await openSecureDb();
  try {
    await new Promise((resolve, reject) => {
      const tx = db.transaction([SECURE_STORE_NAME], 'readwrite');
      tx.objectStore(SECURE_STORE_NAME).put({ id, value });
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error || new Error('secure put failed'));
    });
  } finally {
    db.close();
  }
}

async function secureStoreGet(id) {
  const db = await openSecureDb();
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction([SECURE_STORE_NAME], 'readonly');
      const req = tx.objectStore(SECURE_STORE_NAME).get(id);
      req.onsuccess = () => resolve(req.result ? req.result.value : null);
      req.onerror = () => reject(req.error || new Error('secure get failed'));
    });
  } finally {
    db.close();
  }
}

async function ensureWebAuthnCredential() {
  const existing = localStorage.getItem(WEBAUTHN_CRED_ID_KEY);
  if (existing) return existing;
  if (!hasWebAuthn()) throw new Error('WebAuthn is not available in this browser');
  const rpId = globalThis.location.hostname;
  const userId = randomBytes(16);
  const cred = await navigator.credentials.create({
    publicKey: {
      challenge: randomBytes(32),
      rp: { name: 'Bao WASM Demo', id: rpId },
      user: {
        id: userId,
        name: `bao@${rpId}`,
        displayName: 'Bao User',
      },
      pubKeyCredParams: [{ type: 'public-key', alg: -7 }],
      timeout: 60000,
      attestation: 'none',
      authenticatorSelection: {
        userVerification: 'required',
        residentKey: 'preferred',
      },
    },
  });
  const credId = bytesToBase64Url(new Uint8Array(cred.rawId));
  localStorage.setItem(WEBAUTHN_CRED_ID_KEY, credId);
  localStorage.setItem(WEBAUTHN_ENABLED_KEY, '1');
  return credId;
}

async function verifyUserPresenceWithWebAuthn(credId) {
  if (!hasWebAuthn()) throw new Error('WebAuthn is not available in this browser');
  await navigator.credentials.get({
    publicKey: {
      challenge: randomBytes(32),
      rpId: globalThis.location.hostname,
      allowCredentials: [{ type: 'public-key', id: base64UrlToBytes(credId) }],
      timeout: 60000,
      userVerification: 'required',
    },
  });
}

async function encryptPrivateId(privateId, key) {
  const iv = randomBytes(12);
  const pt = new TextEncoder().encode(privateId);
  const ct = new Uint8Array(await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, pt));
  return {
    v: 1,
    alg: 'AES-GCM',
    iv: bytesToBase64(iv),
    ct: bytesToBase64(ct),
  };
}

async function decryptPrivateId(sealed, key) {
  if (!sealed || !sealed.iv || !sealed.ct) throw new Error('invalid sealed private ID payload');
  const iv = decodeBase64ToBytes(sealed.iv);
  const ct = decodeBase64ToBytes(sealed.ct);
  const pt = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, ct);
  return new TextDecoder().decode(pt);
}

async function sealPrivateIdWithWebAuthn(privateId) {
  const credId = await ensureWebAuthnCredential();
  await verifyUserPresenceWithWebAuthn(credId);
  let key = await secureStoreGet(PRIVATE_ID_KEY_HANDLE);
  if (!key) {
    key = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt']);
    await secureStorePut(PRIVATE_ID_KEY_HANDLE, key);
  }
  const sealed = await encryptPrivateId(privateId, key);
  localStorage.setItem(PRIVATE_ID_SEALED_KEY, JSON.stringify(sealed));
  localStorage.removeItem(PRIVATE_ID_KEY);
  localStorage.setItem(WEBAUTHN_ENABLED_KEY, '1');
}

async function tryUnsealPrivateIdWithWebAuthn() {
  const sealedRaw = localStorage.getItem(PRIVATE_ID_SEALED_KEY);
  const credId = localStorage.getItem(WEBAUTHN_CRED_ID_KEY);
  if (!sealedRaw || !credId) return '';
  const sealed = JSON.parse(sealedRaw);
  await verifyUserPresenceWithWebAuthn(credId);
  const key = await secureStoreGet(PRIVATE_ID_KEY_HANDLE);
  if (!key) throw new Error('secure key not found for private ID');
  return await decryptPrivateId(sealed, key);
}

async function ensureYamlParser() {
  if (yamlApi) return yamlApi;
  const mod = await import('./vendor/yaml/dist/index.js');
  yamlApi = mod;
  return yamlApi;
}

async function parseStorageConfigText(raw) {
  const text = (raw || '').trim();
  if (!text) {
    return null;
  }
  if (text.startsWith('{') || text.startsWith('[')) {
    return JSON.parse(text);
  }
  const y = await ensureYamlParser();
  if (!y || typeof y.parse !== 'function') {
    throw new Error('YAML parser not available in page');
  }
  return y.parse(text);
}

async function run() {
  try {
    const cacheBust = Date.now();
    await BaoDemo.init(`../build/wasm/bao.wasm?v=${cacheBust}`);
    log('WASM loaded');
  } catch (e) {
    log('Failed to load WASM: ' + e);
    return;
  }

  const privateIdEl = document.getElementById('private-id');
  const publicIdEl = document.getElementById('public-id');
  const creatorPublicIdEl = document.getElementById('creator-public-id');
  const urlEl = document.getElementById('url');
  const storageConfigEl = document.getElementById('storage-config');
  const autoOpenEl = document.getElementById('auto-open');
  const dbVaultPathEl = document.getElementById('db-vault-path');
  const dirEl = document.getElementById('dir');
  const dirLimitEl = document.getElementById('dir-limit');
  const uploadFileEl = document.getElementById('upload-file');
  const uploadDestEl = document.getElementById('upload-dest');
  const entriesEl = document.getElementById('entries');
  const attrsViewEl = document.getElementById('attrs-view');
  const breadcrumbsEl = document.getElementById('explorer-breadcrumbs');
  const upBtn = document.getElementById('btn-up');
  const copyLogBtn = document.getElementById('btn-copy-log');
  const tabButtons = Array.from(document.querySelectorAll('[data-tab-target]'));
  const tabPanels = Array.from(document.querySelectorAll('.tab-panel'));
  const replicaDDLEl = document.getElementById('replica-ddl');
  let currentDir = '';
  let lastEntriesByPath = new Map();

  function activateTab(tabId) {
    for (const b of tabButtons) {
      b.classList.toggle('active', b.getAttribute('data-tab-target') === tabId);
    }
    for (const p of tabPanels) {
      p.classList.toggle('active', p.id === tabId);
    }
  }

  for (const b of tabButtons) {
    b.addEventListener('click', () => activateTab(b.getAttribute('data-tab-target')));
  }

  async function downloadFileFromVault(filePath) {
    const path = normalizeDir(filePath);
    if (!path) throw new Error('file path is empty');
    const out = await BaoDemo.read({ path });
    const dataBase64 = (out && out.dataBase64) ? String(out.dataBase64) : '';
    if (!dataBase64) {
      throw new Error(`empty file payload for ${path}`);
    }
    const bytes = decodeBase64ToBytes(dataBase64);
    const blob = new Blob([bytes], { type: 'application/octet-stream' });
    const fileName = (out && out.name) ? String(out.name) : path.split('/').pop() || 'download.bin';
    const objectUrl = URL.createObjectURL(blob);
    try {
      const a = document.createElement('a');
      a.href = objectUrl;
      a.download = fileName;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    } finally {
      URL.revokeObjectURL(objectUrl);
    }
    log(`Downloaded file: ${path} (${bytes.length} bytes)`);
  }

  async function deleteLocalDbByPath(dbPath) {
    const name = (dbPath || '').trim();
    if (!name) throw new Error('db path is empty');
    if (!globalThis.indexedDB) {
      throw new Error('indexedDB API not available');
    }
    await new Promise((resolve, reject) => {
      const req = indexedDB.deleteDatabase(name);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error || new Error('deleteDatabase failed'));
      req.onblocked = () => reject(new Error('deleteDatabase blocked by open connection'));
    });
  }

  async function refreshPublicId() {
    const privateId = (privateIdEl.value || '').trim();
    if (!privateId) {
      publicIdEl.value = '';
      return;
    }
    try {
      if (typeof globalThis.baoPublicID !== 'function') {
        throw new Error('baoPublicID not exported (rebuild WASM with `make wasm`)');
      }
      const out = await BaoDemo.publicId(privateId);
      const publicId = (out?.publicId || '').toString();
      if (!publicId) {
        throw new Error('empty publicId returned by WASM');
      }
      publicIdEl.value = publicId;
    } catch (e) {
      publicIdEl.value = `ERROR: ${String(e?.message || e)}`;
      log('Public ID error:\n' + formatError(e));
    }
  }

  function renderBreadcrumbs() {
    const dir = normalizeDir(currentDir);
    if (!dir) {
      breadcrumbsEl.innerHTML = `<span class="crumb" data-dir="">/</span>`;
      return;
    }
    const parts = dir.split('/');
    const out = [`<span class="crumb" data-dir="">/</span>`];
    let acc = '';
    for (const p of parts) {
      acc = acc ? `${acc}/${p}` : p;
      out.push(` / <span class="crumb" data-dir="${acc}">${p}</span>`);
    }
    breadcrumbsEl.innerHTML = out.join('');
  }

  function isWebAuthnModeEnabled() {
    const enabled = localStorage.getItem(WEBAUTHN_ENABLED_KEY) === '1';
    const hasSealed = !!localStorage.getItem(PRIVATE_ID_SEALED_KEY);
    return enabled || hasSealed;
  }

  async function loadDir(nextDir) {
    try {
      currentDir = normalizeDir(nextDir);
      dirEl.value = currentDir;
      renderBreadcrumbs();
      entriesEl.innerHTML = `<div class="entry"><div></div><div class="entry-dim">Loading...</div><div></div><div></div></div>`;
      const files = await BaoDemo.readDir({
        dir: currentDir,
        limit: parseInt(dirLimitEl.value || '200', 10),
      });
      const list = Array.isArray(files) ? files : [];
      lastEntriesByPath = new Map();
      if (list.length === 0) {
        entriesEl.innerHTML = `<div class="entry"><div></div><div class="entry-dim">Empty</div><div></div><div></div></div>`;
        return;
      }
      entriesEl.innerHTML = list.map((f) => {
        const icon = f.isDir ? '📁' : '📄';
        const nameClass = f.isDir ? 'entry-name-folder' : 'entry-name-file';
        const clickAttr = f.isDir
          ? `data-open-dir="${joinDir(currentDir, f.name)}"`
          : `data-open-file="${joinDir(currentDir, f.name)}"`;
        const size = f.isDir ? '' : formatSize(f.size ?? 0);
        const fullPath = joinDir(currentDir, f.name);
        const attrs = (f && f.attrs) ? f.attrs : {};
        lastEntriesByPath.set(fullPath, f);
        const attrsText = attrs && attrs.isText ? String(attrs.text || '') : '';
        const attrsPreview = attrsText.length > 80 ? `${attrsText.slice(0, 77)}...` : attrsText;
        const attrsTitle = attrsText ? attrsText.replace(/"/g, '&quot;') : '';
        let attrsCell = '';
        if (attrs && attrs.present) {
          const hint = attrs.isText ? attrsPreview : `binary (${attrs.size || 0} B)`;
          attrsCell = `<button class="attrs-btn" data-show-attrs="${fullPath}">attrs</button> <span title="${attrsTitle}">${hint}</span>`;
        }
        return `<div class="entry" ${clickAttr}>
          <div>${icon}</div>
          <div class="${nameClass}" ${clickAttr}>${f.name ?? ''}</div>
          <div class="entry-dim">${size}</div>
          <div class="entry-dim">${formatModTime(f.modTime)}</div>
          <div class="entry-attrs">${attrsCell}</div>
        </div>`;
      }).join('');
    } catch (e) {
      entriesEl.innerHTML = `<div class="entry"><div></div><div class="entry-dim">Error: ${String(e?.message || e)}</div><div></div><div></div></div>`;
      log('Explorer load error:\n' + formatError(e));
    }
  }

  async function firstRunPrivateIdSetup() {
    const hasLegacy = !!localStorage.getItem(PRIVATE_ID_KEY);
    const hasSealed = !!localStorage.getItem(PRIVATE_ID_SEALED_KEY);
    if (hasSealed) return;
    if (hasLegacy) {
      if (!hasWebAuthn()) return;
      const legacy = (localStorage.getItem(PRIVATE_ID_KEY) || '').trim();
      if (!legacy) return;
      await sealPrivateIdWithWebAuthn(legacy);
      log('Migrated private ID from localStorage to WebAuthn protection');
      return;
    }
    if (!hasWebAuthn()) {
      log('WebAuthn unavailable: keeping legacy private ID storage mode');
      return;
    }
    const wantsGenerate = globalThis.confirm('No private ID found.\n\nOK = Generate a new private ID\nCancel = Import existing private ID');
    let privateId = '';
    if (wantsGenerate) {
      const out = await BaoDemo.newPrivateId();
      privateId = (out?.privateId || '').trim();
      if (!privateId) throw new Error('cannot generate private ID');
      log('Generated new private ID (first-run setup)');
    } else {
      const imported = globalThis.prompt('Paste your private ID:');
      privateId = (imported || '').trim();
      if (!privateId) {
        log('First-run private ID setup canceled');
        return;
      }
      log('Imported private ID (first-run setup)');
    }
    await sealPrivateIdWithWebAuthn(privateId);
    log('Private ID saved with WebAuthn protection');
  }

  try {
    await firstRunPrivateIdSetup();

    const hasSealed = !!localStorage.getItem(PRIVATE_ID_SEALED_KEY);
    if (hasSealed && !privateIdEl.value) {
      try {
        privateIdEl.value = await tryUnsealPrivateIdWithWebAuthn();
        log('Unlocked private ID with WebAuthn');
      } catch (e) {
        log('WebAuthn unlock skipped/failed:\n' + formatError(e));
      }
    }

    const saved = localStorage.getItem(PRIVATE_ID_KEY);
    if (saved && !privateIdEl.value && !isWebAuthnModeEnabled()) {
      privateIdEl.value = saved;
      log('Loaded private ID from localStorage (legacy mode)');
    }
    const savedConfig = localStorage.getItem(STORAGE_CONFIG_KEY);
    if (savedConfig && !storageConfigEl.value) {
      storageConfigEl.value = savedConfig;
      log('Loaded storage config from localStorage');
    }
    const savedCreatorPublicId = localStorage.getItem(CREATOR_PUBLIC_ID_KEY);
    if (savedCreatorPublicId && !creatorPublicIdEl.value) {
      creatorPublicIdEl.value = savedCreatorPublicId;
      log('Loaded creator public ID from localStorage');
    }
    const savedReplicaDDL = localStorage.getItem(REPLICA_DDL_KEY);
    if (savedReplicaDDL && replicaDDLEl && !replicaDDLEl.value) {
      replicaDDLEl.value = savedReplicaDDL;
      log('Loaded replica DDL from localStorage');
    }
    const savedAutoOpen = localStorage.getItem(AUTO_OPEN_KEY);
    if (savedAutoOpen !== null) {
      autoOpenEl.checked = savedAutoOpen === '1';
    }
  } catch (e) {
    log('localStorage unavailable: ' + e);
  }

  autoOpenEl.addEventListener('change', () => {
    try {
      localStorage.setItem(AUTO_OPEN_KEY, autoOpenEl.checked ? '1' : '0');
    } catch (_) {}
  });

  // Always derive public ID on startup, even if private ID came
  // from browser autofill or was already present in the input.
  await refreshPublicId();

  document.getElementById('btn-generate-id').onclick = async () => {
    try {
      const out = await BaoDemo.newPrivateId();
      const generated = out?.privateId || '';
      if (!generated) throw new Error('empty privateId');
      privateIdEl.value = generated;
      log('Generated new private ID');
      await refreshPublicId();
    } catch (e) {
      log('Generate ID error:\n' + formatError(e));
    }
  };

  document.getElementById('btn-save-id').onclick = async () => {
    try {
      if (!privateIdEl.value || privateIdEl.value.trim().length === 0) {
        throw new Error('private ID is empty');
      }
      const privateId = privateIdEl.value.trim();
      if (hasWebAuthn()) {
        await sealPrivateIdWithWebAuthn(privateId);
        log('Saved private ID with WebAuthn protection');
      } else {
        localStorage.setItem(PRIVATE_ID_KEY, privateId);
        log('Saved private ID to localStorage (WebAuthn unavailable)');
      }
      await refreshPublicId();
    } catch (e) {
      log('Save ID error:\n' + formatError(e));
    }
  };


  privateIdEl.addEventListener('input', () => {
    void refreshPublicId();
  });

  creatorPublicIdEl.addEventListener('input', () => {
    try {
      const value = (creatorPublicIdEl.value || '').trim();
      if (value) {
        localStorage.setItem(CREATOR_PUBLIC_ID_KEY, value);
      } else {
        localStorage.removeItem(CREATOR_PUBLIC_ID_KEY);
      }
    } catch (_) {}
  });

  storageConfigEl.addEventListener('input', () => {
    try {
      if ((storageConfigEl.value || '').trim()) {
        localStorage.setItem(STORAGE_CONFIG_KEY, storageConfigEl.value);
        log('Auto-saved storage config');
      } else {
        localStorage.removeItem(STORAGE_CONFIG_KEY);
      }
    } catch (_) {}
  });

  if (replicaDDLEl) {
    replicaDDLEl.addEventListener('input', () => {
      try {
        const ddl = (replicaDDLEl.value || '').trim();
        if (ddl) {
          localStorage.setItem(REPLICA_DDL_KEY, replicaDDLEl.value);
        } else {
          localStorage.removeItem(REPLICA_DDL_KEY);
        }
      } catch (_) {}
    });
  }

  // DB controls
  const dbDriverEl = document.getElementById('db-driver');
  const dbPathEl = document.getElementById('db-path');
  const dbKeyEl = document.getElementById('db-key');
  const dbArgsEl = document.getElementById('db-args');
  const dbMaxEl = document.getElementById('db-max');
  const replicaDbPathEl = document.getElementById('replica-db-path');
  const replicaDirEl = document.getElementById('replica-dir');
  const replicaQueryEl = document.getElementById('replica-query');
  const replicaArgsEl = document.getElementById('replica-args');
  const replicaMaxEl = document.getElementById('replica-max');
  const replicaTablesEl = document.getElementById('replica-tables');
  const replicaResultEl = document.getElementById('replica-result');

  async function refreshReplicaTables() {
    const tables = await BaoDemo.replicaTables();
    const list = Array.isArray(tables) ? tables : [];
    if (list.length === 0) {
      replicaTablesEl.innerHTML = '<div class="entry-dim">No tables</div>';
      replicaResultEl.textContent = 'No replica tables yet. Click "Sync Replica" and then refresh tables.';
      return [];
    }
    replicaTablesEl.innerHTML = list.map((t) => `<div class="table-item" data-table="${String(t).replace(/"/g, '&quot;')}">${t}</div>`).join('');
    replicaResultEl.textContent = `Replica tables: ${list.join(', ')}`;
    return list;
  }

  function renderReplicaRows(rows, context) {
    if (!Array.isArray(rows)) {
      replicaResultEl.textContent = JSON.stringify(rows, null, 2);
      return;
    }
    const maxCols = rows.reduce((m, r) => {
      if (!Array.isArray(r)) return m;
      return Math.max(m, r.length);
    }, 0);
    const safeRows = rows.filter((r) => Array.isArray(r));
    if (maxCols === 0 || safeRows.length === 0) {
      const label = context ? `${context}: ` : '';
      replicaResultEl.innerHTML = `<div class="query-meta">${escapeHtml(label)}0 rows</div>`;
      return;
    }
    const header = [];
    for (let i = 0; i < maxCols; i += 1) {
      header.push(`<th>c${i + 1}</th>`);
    }
    const body = safeRows.map((r) => {
      const tds = [];
      for (let i = 0; i < maxCols; i += 1) {
        const v = r[i];
        if (v === null || typeof v === 'undefined') {
          tds.push('<td><span class="query-null">NULL</span></td>');
          continue;
        }
        if (typeof v === 'object') {
          tds.push(`<td>${escapeHtml(JSON.stringify(v))}</td>`);
          continue;
        }
        tds.push(`<td>${escapeHtml(String(v))}</td>`);
      }
      return `<tr>${tds.join('')}</tr>`;
    }).join('');
    const label = context ? `${context} ` : '';
    replicaResultEl.innerHTML = `
      <div class="query-meta">${escapeHtml(label)}${safeRows.length} rows, ${maxCols} columns</div>
      <table class="query-table">
        <thead><tr>${header.join('')}</tr></thead>
        <tbody>${body}</tbody>
      </table>
    `;
  }

  async function previewReplicaTable(tableName) {
    const table = String(tableName || '').trim();
    if (!table) return;
    const max = parseInt(replicaMaxEl.value || '100', 10);
    const rows = await BaoDemo.replicaTablePreview({ table, limit: max });
    replicaQueryEl.value = `SELECT * FROM "${table}" LIMIT ${max}`;
    renderReplicaRows(rows, `Table ${table}`);
  }

  async function openVault(auto = false) {
    await refreshPublicId();
    const creatorPublicId = (creatorPublicIdEl.value || '').trim();
    if (!creatorPublicId) {
      if (!auto) throw new Error('creator public ID is required');
      return false;
    }
    const privateId = (privateIdEl.value || '').trim();
    if (!privateId) {
      if (!auto) throw new Error('private ID is required');
      return false;
    }
    const storageConfig = await parseStorageConfigText(storageConfigEl.value);
    localStorage.setItem(CREATOR_PUBLIC_ID_KEY, creatorPublicId);
    if ((storageConfigEl.value || '').trim()) {
      localStorage.setItem(STORAGE_CONFIG_KEY, storageConfigEl.value);
      log('Auto-saved storage config');
    }
    log('Open request: ' + JSON.stringify({
      hasPrivateId: Boolean(privateId),
      hasCreatorPublicId: Boolean(creatorPublicId),
      hasStorageConfig: Boolean(storageConfig),
      storeUrl: urlEl.value || '',
      dbPath: dbVaultPathEl.value || 'myapp/main.sqlite',
    }));
    const res = await BaoDemo.open({
      privateId,
      author: creatorPublicId,
      storeUrl: urlEl.value,
      storageConfig,
      dbPath: dbVaultPathEl.value || 'myapp/main.sqlite',
    });
    log('Opened vault: ' + JSON.stringify(res));
    await loadDir('');
    return true;
  }

  document.getElementById('btn-open').onclick = async () => {
    try {
      await openVault(false);
    } catch (e) {
      log('Open error:\n' + formatError(e));
    }
  };

  document.getElementById('btn-delete-db').onclick = async () => {
    try {
      const dbPath = dbVaultPathEl.value || 'myapp/main.sqlite';
      try {
        await BaoDemo.close();
      } catch (_) {}
      await deleteLocalDbByPath(dbPath);
      log(`Deleted local DB: ${dbPath}`);
    } catch (e) {
      log('Delete DB error:\n' + formatError(e));
    }
  };

  document.getElementById('btn-sync').onclick = async () => {
    try {
      const res = await BaoDemo.sync();
      log('Sync done: ' + JSON.stringify(res));
      await loadDir(currentDir);
    } catch (e) { log('Sync error:\n' + formatError(e)); }
  };

  document.getElementById('btn-close').onclick = async () => {
    try {
      await BaoDemo.close();
      log('Closed vault');
    } catch (e) { log('Close error:\n' + formatError(e)); }
  };

  document.getElementById('btn-list').onclick = async () => {
    await loadDir(dirEl.value || '');
  };

  document.getElementById('btn-upload').onclick = async () => {
    try {
      const input = uploadFileEl;
      if (!input || !input.files || input.files.length === 0) {
        throw new Error('select a file first');
      }
      const file = input.files[0];
      const bytes = new Uint8Array(await file.arrayBuffer());
      const dataBase64 = bytesToBase64(bytes);
      const name = file.name || 'upload.bin';
      const destination = (uploadDestEl.value || '').trim() || joinDir(currentDir, name);
      if (!destination) {
        throw new Error('destination path is empty');
      }
      log(`Upload request: path=${destination}, size=${bytes.length} B`);
      const res = await BaoDemo.write({
        path: destination,
        dataBase64,
      });
      log('Upload done: ' + JSON.stringify(res));
      await loadDir(currentDir);
    } catch (e) {
      log('Upload error:\n' + formatError(e));
    }
  };

  upBtn.onclick = async () => {
    await loadDir(parentDir(currentDir));
  };

  breadcrumbsEl.addEventListener('click', async (ev) => {
    const target = ev.target;
    if (!(target instanceof HTMLElement)) return;
    const nextDir = target.getAttribute('data-dir');
    if (nextDir === null) return;
    await loadDir(nextDir);
  });

  entriesEl.addEventListener('click', async (ev) => {
    let targetEl = null;
    if (ev.target instanceof HTMLElement) {
      targetEl = ev.target;
    } else if (ev.target && ev.target.parentElement instanceof HTMLElement) {
      targetEl = ev.target.parentElement;
    }
    if (!targetEl) return;
    const clicked = targetEl.closest('[data-open-dir], [data-open-file]');
    const attrsTarget = targetEl.closest('[data-show-attrs]');
    if (attrsTarget instanceof HTMLElement) {
      const filePath = attrsTarget.getAttribute('data-show-attrs');
      if (!filePath) return;
      const file = lastEntriesByPath.get(filePath);
      if (!file || !file.attrs || !file.attrs.present) {
        attrsViewEl.textContent = `No attrs for ${filePath}`;
        return;
      }
      const attrs = file.attrs;
      const lines = [];
      lines.push(`path: ${filePath}`);
      lines.push(`size: ${attrs.size || 0} B`);
      lines.push(`isText: ${Boolean(attrs.isText)}`);
      if (attrs.isText && attrs.text) {
        lines.push('');
        lines.push('text:');
        lines.push(String(attrs.text));
      }
      if (attrs.rawBase64) {
        lines.push('');
        lines.push('rawBase64:');
        lines.push(String(attrs.rawBase64));
      }
      attrsViewEl.textContent = lines.join('\n');
      return;
    }
    if (!(clicked instanceof HTMLElement)) return;
    const nextDir = clicked.getAttribute('data-open-dir');
    if (nextDir) {
      await loadDir(nextDir);
      return;
    }
    const filePath = clicked.getAttribute('data-open-file');
    if (filePath) {
      try {
        await downloadFileFromVault(filePath);
      } catch (e) {
        log('Download error:\n' + formatError(e));
      }
    }
  });

  copyLogBtn.onclick = async () => {
    const text = logBuffer.join('\n');
    if (!text.trim()) {
      console.log('Copy log: nothing to copy');
      return;
    }
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text);
        console.log('Copied log buffer to clipboard');
        return;
      }
    } catch (_) {}
    try {
      const ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
      console.log('Copied log buffer to clipboard');
    } catch (e) {
      console.log('Copy log error:\n' + formatError(e));
    }
  };

  // DB actions
  document.getElementById('btn-db-open').onclick = async () => {
    try {
      const driver = dbDriverEl.value || 'sqlite3';
      const p = dbPathEl.value || 'myapp/main.sqlite';
      await BaoDemo.dbOpen(driver, p);
      log('DB opened: ' + driver + ' ' + p);
    } catch (e) { log('DB open error:\n' + formatError(e)); }
  };

  document.getElementById('btn-db-exec').onclick = async () => {
    try {
      const key = dbKeyEl.value;
      const args = dbArgsEl.value ? JSON.parse(dbArgsEl.value) : {};
      const res = await BaoDemo.dbExec(key, args);
      log('Exec result: ' + JSON.stringify(res));
    } catch (e) { log('DB exec error:\n' + formatError(e)); }
  };

  document.getElementById('btn-db-fetch').onclick = async () => {
    try {
      const key = dbKeyEl.value;
      const args = dbArgsEl.value ? JSON.parse(dbArgsEl.value) : {};
      const max = parseInt(dbMaxEl.value || '100', 10);
      const rows = await BaoDemo.dbFetch(key, args, max);
      log('Fetch rows: ' + JSON.stringify(rows));
    } catch (e) { log('DB fetch error:\n' + formatError(e)); }
  };

  document.getElementById('btn-db-one').onclick = async () => {
    try {
      const key = dbKeyEl.value;
      const args = dbArgsEl.value ? JSON.parse(dbArgsEl.value) : {};
      const row = await BaoDemo.dbFetchOne(key, args);
      log('Fetch one: ' + JSON.stringify(row));
    } catch (e) { log('DB fetchOne error:\n' + formatError(e)); }
  };

  document.getElementById('btn-replica-open').onclick = async () => {
    try {
      const ddl = (replicaDDLEl && replicaDDLEl.value) ? replicaDDLEl.value : '';
      const res = await BaoDemo.replicaOpen({
        dbPath: replicaDbPathEl.value || 'myapp/replica.sqlite',
        dir: replicaDirEl.value || 'replica',
        ddl,
      });
      log('Replica open: ' + JSON.stringify(res));
      const syncRes = await BaoDemo.replicaSync();
      log('Replica sync (after open): ' + JSON.stringify(syncRes));
      const tables = await refreshReplicaTables();
      if (tables.length > 0) {
        await previewReplicaTable(tables[0]);
      }
    } catch (e) {
      log('Replica open error:\n' + formatError(e));
      const msg = String(e?.message || e || '');
      if (msg.includes('query not found')) {
        replicaResultEl.textContent = 'Replica sync failed: missing SQL query keys in replica DB.\nPaste your SQL definitions in "Replica DDL" (for example INSERT_POST and related queries), then click Open Replica again.';
      }
    }
  };

  document.getElementById('btn-replica-sync').onclick = async () => {
    try {
      const res = await BaoDemo.replicaSync();
      log('Replica sync: ' + JSON.stringify(res));
      await refreshReplicaTables();
    } catch (e) {
      log('Replica sync error:\n' + formatError(e));
    }
  };

  document.getElementById('btn-replica-tables').onclick = async () => {
    try {
      await refreshReplicaTables();
    } catch (e) {
      log('Replica tables error:\n' + formatError(e));
    }
  };

  document.getElementById('btn-replica-fetch').onclick = async () => {
    try {
      const q = (replicaQueryEl.value || '').trim();
      if (!q) throw new Error('replica query is empty');
      const args = replicaArgsEl.value ? JSON.parse(replicaArgsEl.value) : {};
      const max = parseInt(replicaMaxEl.value || '100', 10);
      const rows = await BaoDemo.replicaFetch({ query: q, args, max });
      renderReplicaRows(rows, 'Query result');
    } catch (e) {
      log('Replica fetch error:\n' + formatError(e));
    }
  };

  document.getElementById('btn-replica-exec').onclick = async () => {
    try {
      const q = (replicaQueryEl.value || '').trim();
      if (!q) throw new Error('replica query is empty');
      const args = replicaArgsEl.value ? JSON.parse(replicaArgsEl.value) : {};
      const res = await BaoDemo.replicaExec({ query: q, args });
      replicaResultEl.textContent = JSON.stringify(res, null, 2);
      await refreshReplicaTables();
    } catch (e) {
      log('Replica exec error:\n' + formatError(e));
    }
  };

  replicaTablesEl.addEventListener('click', async (ev) => {
    const target = ev.target;
    if (!(target instanceof HTMLElement)) return;
    const table = target.getAttribute('data-table');
    if (!table) return;
    try {
      await previewReplicaTable(table);
    } catch (e) {
      log('Replica preview error:\n' + formatError(e));
    }
  });

  if (autoOpenEl.checked) {
    try {
      const opened = await openVault(true);
      if (opened) {
        log('Auto-open completed');
      }
    } catch (e) {
      log('Auto-open error:\n' + formatError(e));
    }
  }
}

run();
