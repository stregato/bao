// db_worker.js — classic Worker (not a module)
/* eslint-disable no-undef */
"use strict";

let sqlite3, db;
const stmts = new Map();
let nextStmtId = 1;

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
      db.exec({ sql: m.sql, bind: m.args || [] });
      ok(m, {
        result: {
          rowsAffected: db.changes(),
          lastInsertId: db.lastInsertRowid(),
        },
      });
      return;
    }

    /** ENGINE-LEVEL QUERY (all rows) */
    if (m.op === "query") {
      needDb();
      const s = db.prepare(m.sql);
      try {
        s.bind(m.args || []);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const rows = [];
        while (s.step()) rows.push(s.get({ rowMode: "array" }));
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
        s.bind(m.args || []);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const hasRow = s.step();
        const row = hasRow ? s.get({ rowMode: "array" }) : null;
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
        s.bind(m.args || []);
        // step once for DML; for SELECT user should use queryStmt/queryRowStmt
        s.step();
        s.reset();
        ok(m, {
          result: {
            rowsAffected: db.changes(),
            lastInsertId: db.lastInsertRowid(),
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
        s.bind(m.args || []);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const rows = [];
        while (s.step()) rows.push(s.get({ rowMode: "array" }));
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
        s.bind(m.args || []);
        const columns = s.getColumnNames?.() || [];
        const declTypes = declTypesOf(s, sqlite3.capi);
        const hasRow = s.step();
        const row = hasRow ? s.get({ rowMode: "array" }) : null;
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