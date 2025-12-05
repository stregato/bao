// Minimal shim to call into Go WASM exports.
// Assumes the Go build produces functions exported on globalThis.

export const BaoDemo = {
  go: null,
  ready: null,

  async init(wasmPath) {
    if (!('Go' in globalThis)) {
      throw new Error('wasm_exec.js not loaded.');
    }
    this.go = new Go();
    const resp = await fetch(wasmPath);
    const bytes = await resp.arrayBuffer();
    const result = await WebAssembly.instantiate(bytes, this.go.importObject);
    this.ready = this.go.run(result.instance);
    // After run, Go sets up global functions (define in a tiny JS glue in Go if needed)
  },

  // The following functions assume you exposed JS-callable functions from Go via syscall/js
  // For example, in a js-build file, add:
  //   //go:build js
  //   package main
  //   import "syscall/js"
  //   func main() { /* register funcs like js.Global().Set("baoCreate", js.FuncOf(...)) */ }

  async create(url, author) {
    if (!globalThis.baoCreate) throw new Error('baoCreate not exported');
    return await globalThis.baoCreate(url, author);
  },

  async open(url, author) {
    if (!globalThis.baoOpen) throw new Error('baoOpen not exported');
    return await globalThis.baoOpen(url, author);
  },

  async write(path, group, text) {
    if (!globalThis.baoWrite) throw new Error('baoWrite not exported');
    return await globalThis.baoWrite(path, group, text);
  },

  async read(path) {
    if (!globalThis.baoRead) throw new Error('baoRead not exported');
    return await globalThis.baoRead(path);
  },

  async list(dir) {
    if (!globalThis.baoList) throw new Error('baoList not exported');
    return await globalThis.baoList(dir);
  },

  // DB exposed functions
  async dbOpen(driver, dbPath) {
    if (!globalThis.dbOpen) throw new Error('dbOpen not exported');
    return await globalThis.dbOpen(driver, dbPath);
  },
  async dbExec(key, args) {
    if (!globalThis.dbExec) throw new Error('dbExec not exported');
    return await globalThis.dbExec(key, JSON.stringify(args || {}));
  },
  async dbFetch(key, args, maxRows) {
    if (!globalThis.dbFetch) throw new Error('dbFetch not exported');
    return await globalThis.dbFetch(key, JSON.stringify(args || {}), maxRows || 100);
  },
  async dbFetchOne(key, args) {
    if (!globalThis.dbFetchOne) throw new Error('dbFetchOne not exported');
    return await globalThis.dbFetchOne(key, JSON.stringify(args || {}));
  },
};
