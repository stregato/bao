import { BaoDemo } from './shim.js';

const logEl = document.getElementById('log');
function log(msg) { logEl.textContent += `\n${msg}`; }

async function run() {
  try {
    await BaoDemo.init('../build/wasm/bao.wasm');
    log('WASM loaded');
  } catch (e) {
    log('Failed to load WASM: ' + e);
    return;
  }

  const urlEl = document.getElementById('url');
  const authorEl = document.getElementById('author');
  const pathEl = document.getElementById('path');
  const groupEl = document.getElementById('group');
  const contentEl = document.getElementById('content');

  // DB controls
  const dbDriverEl = document.getElementById('db-driver');
  const dbPathEl = document.getElementById('db-path');
  const dbKeyEl = document.getElementById('db-key');
  const dbArgsEl = document.getElementById('db-args');
  const dbMaxEl = document.getElementById('db-max');

  document.getElementById('btn-create').onclick = async () => {
    try {
      await BaoDemo.create(urlEl.value, authorEl.value);
      log('Created bao at ' + urlEl.value);
    } catch (e) { log('Create error: ' + e); }
  };

  document.getElementById('btn-open').onclick = async () => {
    try {
      await BaoDemo.open(urlEl.value, authorEl.value);
      log('Opened bao at ' + urlEl.value);
    } catch (e) { log('Open error: ' + e); }
  };

  document.getElementById('btn-write').onclick = async () => {
    try {
      await BaoDemo.write(pathEl.value, groupEl.value, contentEl.value);
      log('Wrote ' + pathEl.value);
    } catch (e) { log('Write error: ' + e); }
  };

  document.getElementById('btn-read').onclick = async () => {
    try {
      const text = await BaoDemo.read(pathEl.value);
      log('Read ' + pathEl.value + ': ' + text);
    } catch (e) { log('Read error: ' + e); }
  };

  document.getElementById('btn-list').onclick = async () => {
    try {
      const files = await BaoDemo.list('/');
      log('Dir: ' + JSON.stringify(files));
    } catch (e) { log('List error: ' + e); }
  };

  // DB actions
  document.getElementById('btn-db-open').onclick = async () => {
    try {
      const driver = dbDriverEl.value || 'sqlite3';
      const p = dbPathEl.value || 'myapp/main.sqlite';
      await BaoDemo.dbOpen(driver, p);
      log('DB opened: ' + driver + ' ' + p);
    } catch (e) { log('DB open error: ' + e); }
  };

  document.getElementById('btn-db-exec').onclick = async () => {
    try {
      const key = dbKeyEl.value;
      const args = dbArgsEl.value ? JSON.parse(dbArgsEl.value) : {};
      const res = await BaoDemo.dbExec(key, args);
      log('Exec result: ' + JSON.stringify(res));
    } catch (e) { log('DB exec error: ' + e); }
  };

  document.getElementById('btn-db-fetch').onclick = async () => {
    try {
      const key = dbKeyEl.value;
      const args = dbArgsEl.value ? JSON.parse(dbArgsEl.value) : {};
      const max = parseInt(dbMaxEl.value || '100', 10);
      const rows = await BaoDemo.dbFetch(key, args, max);
      log('Fetch rows: ' + JSON.stringify(rows));
    } catch (e) { log('DB fetch error: ' + e); }
  };

  document.getElementById('btn-db-one').onclick = async () => {
    try {
      const key = dbKeyEl.value;
      const args = dbArgsEl.value ? JSON.parse(dbArgsEl.value) : {};
      const row = await BaoDemo.dbFetchOne(key, args);
      log('Fetch one: ' + JSON.stringify(row));
    } catch (e) { log('DB fetchOne error: ' + e); }
  };
}

run();
