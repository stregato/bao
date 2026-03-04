// db_worker.js — classic Worker (not a module)
/* eslint-disable no-undef */
"use strict";

let sqlite3, db;
const stmts = new Map();
let nextStmtId = 1;

function isByteArrayLike(v) {
  if (!(Array.isArray(v))) return false;
  for (let i = 0; i < v.length; i++) {
    const n = v[i];
    if (typeof n !== "number" || !Number.isInteger(n) || n < 0 || n > 255) {
      return false;
    }
  }
  return true;
}

function normalizeBindValue(v) {
  if (isByteArrayLike(v)) {
    return Uint8Array.from(v);
  }
  return v;
}

function normalizeBindArgs(args) {
  if (Array.isArray(args)) {
    return args.map(normalizeBindValue);
  }
  if (args && typeof args === "object") {
    const out = {};
    for (const [k, v] of Object.entries(args)) {
      out[k] = normalizeBindValue(v);
    }
    return out;
  }
  return args;
}

function bindIfPresent(stmt, args) {
  const normalized = normalizeBindArgs(args);
  if (Array.isArray(args) && args.length > 0) {
    stmt.bind(normalized);
    return;
  }
  if (normalized && typeof normalized === "object" && !Array.isArray(normalized)) {
    const keys = Object.keys(normalized);
    if (keys.length > 0) {
      // sqlite wasm can reject extra keys that are not real SQL bind params.
      // Keep removing invalid names and retry to mirror native sqlite behavior.
      const attempt = { ...normalized };
      while (true) {
        try {
          stmt.bind(attempt);
          break;
        } catch (e) {
          const msg = String(e);
          const m = msg.match(/Invalid bind\(\) parameter name:\s*([^\s]+)/i);
          if (!m || !m[1]) {
            const details = `bind object failed, keys=${JSON.stringify(Object.keys(attempt))}, err=${msg}`;
            throw new Error(details);
          }
          let bad = m[1].trim();
          // Normalize "id" vs ":id"/"$id"/"@id" forms.
          const variants = [bad];
          if (!bad.startsWith(":") && !bad.startsWith("$") && !bad.startsWith("@")) {
            variants.push(":" + bad, "$" + bad, "@" + bad);
          } else {
            const bare = bad.slice(1);
            variants.push(bare, ":" + bare, "$" + bare, "@" + bare);
          }
          let removed = false;
          for (const k of variants) {
            if (Object.prototype.hasOwnProperty.call(attempt, k)) {
              delete attempt[k];
              removed = true;
            }
          }
          if (!removed) {
            const details = `bind object failed, keys=${JSON.stringify(Object.keys(attempt))}, err=${msg}`;
            throw new Error(details);
          }
          if (Object.keys(attempt).length === 0) {
            const details = `bind object resolved to empty map after removing invalid key ${bad}; original keys may not match SQL parameters`;
            throw new Error(details);
          }
        }
      }
    }
  }
}

function normalizeRow(row, columnCount) {
  let out = row;
  if (Array.isArray(out) && out.length === columnCount + 1 && out[0] === "array") {
    out = out.slice(1);
  }
  if (Array.isArray(out)) {
    for (let i = 0; i < out.length; i++) {
      // Go syscall/js does not handle JS BigInt reliably across all paths.
      // Convert to decimal string so Go scanner can parse to int64/uint64.
      if (typeof out[i] === "bigint") {
        out[i] = out[i].toString();
      }
    }
  }
  return out;
}

function safeChanges() {
  try {
    if (db && typeof db.changes === "function") {
      return Number(db.changes()) || 0;
    }
  } catch (_) {}
  return 0;
}

function safeLastInsertId() {
  try {
    if (db && typeof db.lastInsertRowid === "function") {
      return Number(db.lastInsertRowid()) || 0;
    }
  } catch (_) {}
  try {
    if (db && typeof db.selectValue === "function") {
      return Number(db.selectValue("SELECT last_insert_rowid()")) || 0;
    }
  } catch (_) {}
  return 0;
}

/** Utilities */
function ok(m, extra = {}) {
  postMessage({ reqId: m.reqId ?? null, ok: true, ...extra });
}
function fail(m, err) {
  postMessage({ reqId: m.reqId ?? null, ok: false, err: String(err) });
}
function needDb() {
  if (!db) throw new Error("database not open");
}

/** Declared types helper (schema types, not runtime storage class) */
function declTypesOf(stmt, capi) {
  const names = stmt.getColumnNames?.() || [];
  const out = new Array(names.length);
  for (let i = 0; i < names.length; i++) {
    // NULL if column is an expression; normalize to empty string
    const t = capi.sqlite3_column_decltype(stmt, i);
    out[i] = t ? String(t).toUpperCase() : "";
  }
  return out;
}

self.onmessage = async (e) => {
  const m = e.data || {};
  try {
    /** OPEN */
    if (m.op === "open") {
      if (!sqlite3) {
        // Load official bundle; it will fetch sqlite3.wasm as needed.
        importScripts("./sqlite3.js");
        sqlite3 = await self.sqlite3InitModule();
      }
      const path = m.path || "myapp/main.sqlite";
      const hasOPFS = !!sqlite3.capi.sqlite3_vfs_find?.("opfs");
      const uri = hasOPFS ? `file:/${path}?vfs=opfs` : ":memory:";
      db = new sqlite3.oo1.DB(uri);
      // WAL often improves write performance; ignore errors if unsupported.
      try { db.exec("PRAGMA journal_mode=WAL;"); } catch (_) {}
      ok(m, { vfs: hasOPFS ? "opfs" : "memory" });
      return;
    }

    /** PREPARE (optionally under a tx; txId is cosmetic in single-conn model) */
    if (m.op === "prepare" || m.op === "prepareTx") {
      needDb();
      const s = db.prepare(m.sql);
      const id = nextStmtId++;
      stmts.set(id, s);
      ok(m, { stmtId: id });
      return;
    }

    /** CLOSE STMT */
    if (m.op === "closeStmt") {
      const s = stmts.get(m.stmtId);
      if (s) {
        try { s.finalize(); } catch (_) {}
        stmts.delete(m.stmtId);
      }
      ok(m, {});
      return;
    }

    /** ENGINE-LEVEL EXEC (no prepared handle) */
    if (m.op === "exec") {
      needDb();
      const s = db.prepare(m.sql);
      try {
        s.reset();
        bindIfPresent(s, m.args);
        s.step();
        s.reset();
        ok(m, {
          result: {
            rowsAffected: safeChanges(),
            lastInsertId: safeLastInsertId(),
          },
        });
      } catch (e) {
        try { s.reset(); } catch (_) {}
        const preview = (() => {
          try {
            const txt = JSON.stringify(m.args);
            return txt.length > 400 ? txt.slice(0, 400) + "..." : txt;
          } catch (_) {
            return String(m.args);
          }
        })();
        fail(m, `exec failed, sql=${String(m.sql)}, args=${preview}, err=${String(e)}`);
      } finally {
        try { s.finalize(); } catch (_) {}
      }
      return;
    }

    /** ENGINE-LEVEL QUERY (all rows) */
    if (m.op === "query") {
      needDb();
      const s = db.prepare(m.sql);
      try {
        bindIfPresent(s, m.args);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const rows = [];
        while (s.step()) {
          rows.push(normalizeRow(s.get([]), columns.length));
        }
        s.reset();
        ok(m, { columns, declTypes, rows });
      } finally {
        try { s.finalize(); } catch (_) {}
      }
      return;
    }

    /** ENGINE-LEVEL QUERY ONE */
    if (m.op === "queryRow") {
      needDb();
      const s = db.prepare(m.sql);
      try {
        bindIfPresent(s, m.args);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const hasRow = s.step();
        const row = hasRow ? normalizeRow(s.get([]), columns.length) : null;
        s.reset();
        ok(m, { columns, declTypes, hasRow, row });
      } finally {
        try { s.finalize(); } catch (_) {}
      }
      return;
    }

    /** STMT-LEVEL EXEC */
    if (m.op === "execStmt") {
      needDb();
      const s = stmts.get(m.stmtId);
      if (!s) return fail(m, "invalid stmtId");
      try {
        s.reset();
        bindIfPresent(s, m.args);
        // step once for DML; for SELECT user should use queryStmt/queryRowStmt
        s.step();
        s.reset();
        ok(m, {
          result: {
            rowsAffected: safeChanges(),
            lastInsertId: safeLastInsertId(),
          },
        });
      } catch (e) {
        try { s.reset(); } catch (_) {}
        fail(m, e);
      }
      return;
    }

    /** STMT-LEVEL QUERY (all rows) */
    if (m.op === "queryStmt") {
      needDb();
      const s = stmts.get(m.stmtId);
      if (!s) return fail(m, "invalid stmtId");
      try {
        s.reset();
        bindIfPresent(s, m.args);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const rows = [];
        while (s.step()) {
          rows.push(normalizeRow(s.get([]), columns.length));
        }
        s.reset();
        ok(m, { columns, declTypes, rows });
      } catch (e) {
        try { s.reset(); } catch (_) {}
        fail(m, e);
      }
      return;
    }

    /** STMT-LEVEL QUERY ONE */
    if (m.op === "queryRowStmt") {
      needDb();
      const s = stmts.get(m.stmtId);
      if (!s) return fail(m, "invalid stmtId");
      try {
        s.reset();
        bindIfPresent(s, m.args);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const hasRow = s.step();
        const row = hasRow ? normalizeRow(s.get([]), columns.length) : null;
        s.reset();
        ok(m, { columns, declTypes, hasRow, row });
      } catch (e) {
        try { s.reset(); } catch (_) {}
        fail(m, e);
      }
      return;
    }

    /** TRANSACTIONS (single-connection; txId is informational) */
    if (m.op === "begin")    { needDb(); db.exec("BEGIN IMMEDIATE;"); ok(m, { txId: 1 }); return; }
    if (m.op === "commit")   { needDb(); db.exec("COMMIT;"); ok(m, {}); return; }
    if (m.op === "rollback") { needDb(); db.exec("ROLLBACK;"); ok(m, {}); return; }

    /** CLOSE (finalize everything, checkpoint WAL, close DB, terminate worker) */
    if (m.op === "close") {
      for (const s of stmts.values()) { try { s.finalize(); } catch (_) {} }
      stmts.clear();
      if (db) {
        try { db.exec("PRAGMA wal_checkpoint(TRUNCATE);"); } catch (_) {}
        try { db.close(); } catch (_) {}
        db = null;
      }
      ok(m, {});
      // allow the ok() message to flush before closing the worker
      setTimeout(() => self.close(), 0);
      return;
    }

    // Unknown op → ignore politely or fail.
    fail(m, `unknown op: ${m.op}`);
  } catch (err) {
    fail(m, err);
  }
};
