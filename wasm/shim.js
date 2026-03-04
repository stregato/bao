// Minimal shim to call into Go WASM exports.
// Assumes the Go build produces functions exported on globalThis.

export const BaoDemo = {
  go: null,
  ready: null,

  async _waitForExports(names, timeoutMs = 4000) {
    const started = Date.now();
    while (true) {
      const allReady = names.every((n) => typeof globalThis[n] === 'function');
      if (allReady) return;
      if (Date.now() - started > timeoutMs) {
        const missing = names.filter((n) => typeof globalThis[n] !== 'function');
        throw new Error(`WASM exports not ready: ${missing.join(', ')}`);
      }
      await new Promise((r) => setTimeout(r, 25));
    }
  },

  _decode(result) {
    if (typeof result !== 'string') return result;
    const text = result.trim();
    if (!text) return result;
    const first = text[0];
    if (first !== '{' && first !== '[') return result;
    try {
      return JSON.parse(text);
    } catch (_) {
      return result;
    }
  },

  async init(wasmPath) {
    if (!('Go' in globalThis)) {
      throw new Error('wasm_exec.js not loaded.');
    }
    this.go = new Go();
    const resp = await fetch(wasmPath);
    const bytes = await resp.arrayBuffer();
    const result = await WebAssembly.instantiate(bytes, this.go.importObject);
    this.ready = this.go.run(result.instance);
    // Wait until exports are actually registered by Go runtime.
    await this._waitForExports(['baoNewPrivateID', 'baoOpen', 'baoReadDir', 'baoPublicID']);
  },

  // The following functions assume you exposed JS-callable functions from Go via syscall/js
  // For example, in a js-build file, add:
  //   //go:build js
  //   package main
  //   import "syscall/js"
  //   func main() { /* register funcs like js.Global().Set("baoCreate", js.FuncOf(...)) */ }

  async newPrivateId() {
    if (!globalThis.baoNewPrivateID) throw new Error('baoNewPrivateID not exported');
    return this._decode(await globalThis.baoNewPrivateID());
  },

  async publicId(privateId) {
    if (!globalThis.baoPublicID) throw new Error('baoPublicID not exported');
    return this._decode(await globalThis.baoPublicID(JSON.stringify({ privateId: privateId || '' })));
  },

  async create(options) {
    if (!globalThis.baoCreate) throw new Error('baoCreate not exported');
    return this._decode(await globalThis.baoCreate(JSON.stringify(options || {})));
  },

  async open(options) {
    if (!globalThis.baoOpen) throw new Error('baoOpen not exported');
    return this._decode(await globalThis.baoOpen(JSON.stringify(options || {})));
  },

  async sync() {
    if (!globalThis.baoSync) throw new Error('baoSync not exported');
    return this._decode(await globalThis.baoSync());
  },

  async readDir(options) {
    if (!globalThis.baoReadDir) throw new Error('baoReadDir not exported');
    return this._decode(await globalThis.baoReadDir(JSON.stringify(options || {})));
  },

  async close() {
    if (!globalThis.baoClose) throw new Error('baoClose not exported');
    return this._decode(await globalThis.baoClose());
  },

  async write(pathOrOptions, group, text) {
    if (!globalThis.baoWrite) throw new Error('baoWrite not exported');
    if (typeof pathOrOptions === 'string' && arguments.length >= 3) {
      // Backward-compatible legacy call signature.
      return this._decode(await globalThis.baoWrite(JSON.stringify({
        path: pathOrOptions,
        dataBase64: btoa(text || ''),
      })));
    }
    if (typeof pathOrOptions === 'string') {
      return this._decode(await globalThis.baoWrite(pathOrOptions));
    }
    return this._decode(await globalThis.baoWrite(JSON.stringify(pathOrOptions || {})));
  },

  async read(pathOrOptions) {
    if (!globalThis.baoRead) throw new Error('baoRead not exported');
    if (typeof pathOrOptions === 'string') {
      return this._decode(await globalThis.baoRead(pathOrOptions));
    }
    return this._decode(await globalThis.baoRead(JSON.stringify(pathOrOptions || {})));
  },

  async list(dir) {
    if (!globalThis.baoList) throw new Error('baoList not exported');
    return this._decode(await globalThis.baoList(dir));
  },

  async replicaOpen(options) {
    if (!globalThis.replicaOpen) throw new Error('replicaOpen not exported');
    return this._decode(await globalThis.replicaOpen(JSON.stringify(options || {})));
  },
  async replicaSync() {
    if (!globalThis.replicaSync) throw new Error('replicaSync not exported');
    return this._decode(await globalThis.replicaSync());
  },
  async replicaFetch(options) {
    if (!globalThis.replicaFetch) throw new Error('replicaFetch not exported');
    return this._decode(await globalThis.replicaFetch(JSON.stringify(options || {})));
  },
  async replicaExec(options) {
    if (!globalThis.replicaExec) throw new Error('replicaExec not exported');
    return this._decode(await globalThis.replicaExec(JSON.stringify(options || {})));
  },
  async replicaTables() {
    if (!globalThis.replicaTables) throw new Error('replicaTables not exported');
    return this._decode(await globalThis.replicaTables());
  },
  async replicaTablePreview(options) {
    if (!globalThis.replicaTablePreview) throw new Error('replicaTablePreview not exported');
    return this._decode(await globalThis.replicaTablePreview(JSON.stringify(options || {})));
  },

  // DB exposed functions
  async dbOpen(driver, dbPath) {
    if (!globalThis.dbOpen) throw new Error('dbOpen not exported');
    return this._decode(await globalThis.dbOpen(driver, dbPath));
  },
  async dbExec(key, args) {
    if (!globalThis.dbExec) throw new Error('dbExec not exported');
    return this._decode(await globalThis.dbExec(key, JSON.stringify(args || {})));
  },
  async dbFetch(key, args, maxRows) {
    if (!globalThis.dbFetch) throw new Error('dbFetch not exported');
    return this._decode(await globalThis.dbFetch(key, JSON.stringify(args || {}), maxRows || 100));
  },
  async dbFetchOne(key, args) {
    if (!globalThis.dbFetchOne) throw new Error('dbFetchOne not exported');
    return this._decode(await globalThis.dbFetchOne(key, JSON.stringify(args || {})));
  },
};
