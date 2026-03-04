#!/usr/bin/env node
"use strict";

const fs = require("node:fs");

let YAML = null;
try {
  YAML = require("yaml");
} catch (_) {
  // Offline fallback: reuse dependency already available in this workspace.
  YAML = require("/Users/ea/Documents/Lab/section31/magpie/server/magpie-server/node_modules/yaml");
}

const DEFAULT_TTL = 15 * 60;
const MAX_TTL = 7 * 24 * 60 * 60; // 7 days
const API_BASE = "https://api.cloudflare.com/client/v4";
const DEFAULT_PERMISSION = "object-read-write";
const DEFAULT_FORMAT = "yaml";
const DEFAULT_PROXY = "http://localhost:8000/__proxy";
const WRANGLER_TOKEN_PATHS = [];

function parseArgs(argv) {
  const args = {};
  for (let i = 2; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith("--")) continue;
    const key = a.slice(2);
    const next = argv[i + 1];
    if (!next || next.startsWith("--")) {
      args[key] = "true";
      continue;
    }
    args[key] = next;
    i++;
  }
  return args;
}

function required(name, value) {
  if (!value) {
    throw new Error(`missing required value: ${name}`);
  }
  return value;
}

function toInt(v, fallback) {
  const n = Number.parseInt(String(v ?? ""), 10);
  return Number.isFinite(n) ? n : fallback;
}

function splitCsv(v) {
  if (!v) return [];
  return String(v)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

function accountIdFromEndpoint(endpoint) {
  if (!endpoint) return "";
  try {
    const u = new URL(endpoint);
    const host = u.hostname || "";
    const suffix = ".r2.cloudflarestorage.com";
    const euSuffix = ".eu.r2.cloudflarestorage.com";
    const fedSuffix = ".fedramp.r2.cloudflarestorage.com";
    if (host.endsWith(suffix)) return host.slice(0, -suffix.length);
    if (host.endsWith(euSuffix)) return host.slice(0, -euSuffix.length);
    if (host.endsWith(fedSuffix)) return host.slice(0, -fedSuffix.length);
  } catch (_) {}
  return "";
}

function normalizePrefix(p) {
  return String(p || "").replace(/^\/+|\/+$/g, "");
}

function readYamlStorageConfig(configPath, wantedId) {
  const raw = fs.readFileSync(configPath, "utf8");
  const parsed = YAML.parse(raw);
  const list = Array.isArray(parsed) ? parsed : [parsed];
  const stores = list.filter((x) => x && typeof x === "object");
  if (!stores.length) {
    throw new Error("storage config is empty");
  }

  let selected = null;
  if (wantedId) {
    selected = stores.find((s) => String(s.id || "") === wantedId);
    if (!selected) {
      throw new Error(`no store with id '${wantedId}' in config`);
    }
  } else {
    selected = stores.find((s) => String(s.type || "").toLowerCase() === "s3");
    if (!selected) {
      throw new Error("no s3 store found in config; provide --id");
    }
  }

  if (String(selected.type || "").toLowerCase() !== "s3") {
    throw new Error(`selected store '${selected.id || ""}' is not type s3`);
  }
  if (!selected.s3 || typeof selected.s3 !== "object") {
    throw new Error(`selected store '${selected.id || ""}' has invalid s3 section`);
  }
  return selected;
}

function assertPermission(v) {
  const p = String(v || "").trim();
  const allowed = new Set([
    "admin-read-write",
    "admin-read-only",
    "object-read-write",
    "object-read-only",
  ]);
  if (!allowed.has(p)) {
    throw new Error(
      `invalid --permission '${p}' (allowed: ${Array.from(allowed).join(", ")})`
    );
  }
  return p;
}

function buildEndpoint(accountId, sourceEndpoint) {
  if (!sourceEndpoint) return `https://${accountId}.r2.cloudflarestorage.com`;
  try {
    const u = new URL(sourceEndpoint);
    const host = u.hostname || "";
    if (host.includes(".eu.r2.cloudflarestorage.com")) {
      return `https://${accountId}.eu.r2.cloudflarestorage.com`;
    }
    if (host.includes(".fedramp.r2.cloudflarestorage.com")) {
      return `https://${accountId}.fedramp.r2.cloudflarestorage.com`;
    }
  } catch (_) {}
  return `https://${accountId}.r2.cloudflarestorage.com`;
}

function firstExisting(paths) {
  for (const p of paths) {
    try {
      if (fs.existsSync(p)) return p;
    } catch (_) {}
  }
  return "";
}

function readWranglerOAuthToken() {
  const home = process.env.HOME || "";
  const wranglerHome = process.env.WRANGLER_HOME || "";
  const candidates = [];

  if (wranglerHome) {
    candidates.push(`${wranglerHome}/config/default.toml`);
  }
  if (home) {
    candidates.push(`${home}/.wrangler/config/default.toml`);
    candidates.push(`${home}/Library/Preferences/.wrangler/config/default.toml`);
  }
  for (const p of WRANGLER_TOKEN_PATHS) candidates.push(p);

  const file = firstExisting(candidates);
  if (!file) return { token: "", source: "" };

  const raw = fs.readFileSync(file, "utf8");
  const m = raw.match(/^\s*oauth_token\s*=\s*"([^"]+)"/m);
  if (!m || !m[1]) return { token: "", source: "" };
  return { token: m[1], source: `wrangler:${file}` };
}

async function main() {
  const args = parseArgs(process.argv);

  let accountId = args.account || process.env.CF_ACCOUNT_ID || "";
  let bucket = args.bucket || process.env.R2_BUCKET || "";
  let prefix = args.prefix || process.env.R2_PREFIX || "";
  let parentAccessKeyId =
    args["parent-access-key"] || process.env.R2_PARENT_ACCESS_KEY_ID || "";
  let proxy = args.proxy || process.env.R2_PROXY || "";
  const wrangler = readWranglerOAuthToken();
  let apiTokenSource = "";
  const apiToken =
    (args["api-token"] && (apiTokenSource = "--api-token", args["api-token"])) ||
    (process.env.CF_API_TOKEN && (apiTokenSource = "CF_API_TOKEN", process.env.CF_API_TOKEN)) ||
    (process.env.CLOUDFLARE_API_TOKEN && (apiTokenSource = "CLOUDFLARE_API_TOKEN", process.env.CLOUDFLARE_API_TOKEN)) ||
    (wrangler.token && (apiTokenSource = wrangler.source, wrangler.token)) ||
    "";
  const permission = assertPermission(
    args.permission || process.env.R2_TEMP_PERMISSION || DEFAULT_PERMISSION
  );
  const format = String(args.format || process.env.R2_TEMP_FORMAT || DEFAULT_FORMAT).trim();
  const ttlSeconds = Math.max(
    1,
    Math.min(toInt(args.ttl || process.env.R2_TEMP_TTL, DEFAULT_TTL), MAX_TTL)
  );
  let sourceEndpoint = "";

  if (args.config) {
    const cfg = readYamlStorageConfig(args.config, args.id);
    sourceEndpoint = cfg.s3.endpoint || "";
    accountId = accountId || accountIdFromEndpoint(cfg.s3.endpoint);
    bucket = bucket || cfg.s3.bucket || "";
    prefix = prefix || cfg.s3.prefix || "";
    parentAccessKeyId =
      parentAccessKeyId || (cfg.s3.auth && cfg.s3.auth.accessKeyId) || "";
    proxy = proxy || cfg.s3.proxy || "";
  }

  proxy = String(proxy || DEFAULT_PROXY).trim();

  accountId = required("account", accountId);
  bucket = required("bucket", bucket);
  parentAccessKeyId = required("parent-access-key", parentAccessKeyId);
  required("api-token", apiToken);

  const prefixes = splitCsv(args.prefixes);
  const objects = splitCsv(args.objects);
  const effectivePrefix = normalizePrefix(prefix);
  if (effectivePrefix && prefixes.length === 0 && objects.length === 0) {
    prefixes.push(effectivePrefix.endsWith("/") ? effectivePrefix : `${effectivePrefix}/`);
  }

  const body = {
    bucket,
    parentAccessKeyId,
    permission,
    ttlSeconds,
  };
  if (prefixes.length > 0) body.prefixes = prefixes;
  if (objects.length > 0) body.objects = objects;

  const res = await fetch(
    `${API_BASE}/accounts/${encodeURIComponent(accountId)}/r2/temp-access-credentials`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${apiToken}`,
      },
      body: JSON.stringify(body),
    }
  );

  let payload = null;
  try {
    payload = await res.json();
  } catch (_) {
    payload = null;
  }

  if (!res.ok || !payload || payload.success === false) {
    const msg = payload && Array.isArray(payload.errors) && payload.errors.length
      ? payload.errors.map((e) => e.message || JSON.stringify(e)).join("; ")
      : `HTTP ${res.status}`;
    if (/auth/i.test(msg) && apiTokenSource.startsWith("wrangler:")) {
      throw new Error(
        `cloudflare temp credential request failed: ${msg}. Wrangler oauth token is not accepted here; use CF_API_TOKEN or --api-token`
      );
    }
    throw new Error(`cloudflare temp credential request failed: ${msg}`);
  }

  const result = payload.result || {};
  const accessKeyId = required("result.accessKeyId", result.accessKeyId);
  const secretAccessKey = required("result.secretAccessKey", result.secretAccessKey);
  const sessionToken = required("result.sessionToken", result.sessionToken);
  const expiresAt = new Date(Date.now() + ttlSeconds * 1000).toISOString();
  const endpoint = buildEndpoint(accountId, sourceEndpoint);
  const normalizedPrefix = normalizePrefix(prefix);

  const rawOutput = {
    endpoint,
    proxy,
    region: "auto",
    bucket,
    prefix: normalizedPrefix,
    permission,
    ttlSeconds,
    expiresAt,
    credentials: {
      accessKeyId,
      secretAccessKey,
      sessionToken,
    },
    env: {
      AWS_ACCESS_KEY_ID: accessKeyId,
      AWS_SECRET_ACCESS_KEY: secretAccessKey,
      AWS_SESSION_TOKEN: sessionToken,
      AWS_REGION: "auto",
      AWS_DEFAULT_REGION: "auto",
      AWS_ENDPOINT_URL_S3: endpoint,
      R2_BUCKET: bucket,
      R2_PREFIX: normalizedPrefix,
    },
    request: body,
  };

  if (format === "raw") {
    console.log(JSON.stringify(rawOutput, null, 2));
    return;
  }

  const storageConfig = {
    id: args["store-id"] || args.id || "r2-temp",
    type: "s3",
    s3: {
      endpoint,
      proxy,
      region: "auto",
      bucket,
      prefix: normalizedPrefix,
      auth: {
        accessKeyId,
        secretAccessKey,
        sessionToken,
      },
    },
  };

  if (format === "json" || format === "storage-config") {
    console.log(JSON.stringify(storageConfig, null, 2));
    return;
  }
  if (format === "yaml" || format === "storage-config-yaml") {
    console.log(YAML.stringify(storageConfig));
    return;
  }
  throw new Error("invalid --format (allowed: yaml, json, raw)");
}

main().catch((err) => {
  console.error("Error:", err.message);
  console.error("");
  console.error("Usage:");
  console.error(
    "  node scripts/r2-temp-creds.js --config <storage.yaml> [--id <storeId>] [--proxy <URL>] [--ttl 900] [--permission object-read-write] [--prefixes p1/,p2/] [--objects path/a,path/b] [--format yaml|json|raw]"
  );
  console.error(
    "  node scripts/r2-temp-creds.js --account <ACCOUNT_ID> --bucket <BUCKET> --parent-access-key <ACCESS_KEY_ID> --api-token <CF_API_TOKEN> [--prefix <PREFIX>] [--proxy <URL>] [--ttl 900] [--permission object-read-write] [--format yaml|json|raw]"
  );
  console.error("");
  console.error("Notes:");
  console.error("  - default output is StorageConfig YAML; use --format json or --format raw as needed.");
  console.error(`  - proxy precedence: --proxy, R2_PROXY, input config s3.proxy, default ${DEFAULT_PROXY}.`);
  console.error("  - auth lookup order: --api-token, CF_API_TOKEN/CLOUDFLARE_API_TOKEN, Wrangler oauth_token.");
  console.error("  - --parent-access-key is the existing R2 access key ID used as parent.");
  process.exit(1);
});
