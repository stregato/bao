# Bao Repository Documentation

This repository hosts the cross-platform **Bao** secure storage engine, its Go core, bindings for Python/Dart/WASM, and the build scripts that generate shared libraries for desktop and mobile runtimes. The Go library is documented in detail inside `lib/README.md`; this document connects the rest of the tree, explains the higher-level concepts, and points to the key functions and tests that drive each behavior.

## 1. Repository layout & build system

| Path | Purpose |
|------|---------|
| `lib/` | Go implementation of the bao engine, cryptography helpers, storage backends, SQL layer, and CGO exports (see §2). |
| `bindings/py/` | Python package (`pbao`) that loads the compiled library via `ctypes` and exposes an idiomatic API (see §3.1). |
| `bindings/dart/` | Flutter/Dart bindings that mirror the Go API via FFI (see §3.2). |
| `wasm/` + `lib/jsdemo/` | WASM demo that builds the Go code for the browser and exposes JS shims (see §3.3). |
| `build/` | Output artifacts (shared libraries, WASM binaries, xcframework). Built by the Makefile. |
| `module.modulemap` | Allows Swift/Objective-C consumers to import the generated xcframework. |
| `Makefile` | Orchestrates cross-compilation for Linux, macOS (Intel/ARM), Windows, Android, iOS, and WASM, then triggers the language-specific packaging steps (§4).

The **Makefile targets** (`Makefile:1-160`) set `GOOS/GOARCH` and toolchains to build `lib/export.go` as shared libraries. Targets such as `mac`, `ios`, `linux`, `windows`, `android`, and `wasm` emit platform-specific artifacts, while `py`, `java`, and `dart` reuse those artifacts when packaging bindings. The helper target `build_ios_static` (`Makefile:55-65`) configures bitcode-friendly flags for xcframework creation, and `release` (`Makefile:152-160`) zips each platform output and tags a Git release. The WASM build reuses `lib/jsdemo/main_js.go` (`Makefile:45-48`).

## 2. Go core concepts (lib/)

Detailed package-level notes are in `lib/README.md`; here are the foundational ideas with references:

### 2.1 Runtime, logging, and errors
- `core.LogFormatter` configures logrus with elapsed-time columns and an HTTP mirror (`lib/core/log.go:17-100`).
- Structured errors are created via `core.Errorw` (`lib/core/errors.go:78`), which captures file/line metadata that bindings decode (see Dart loader in §3.2).
- `core.Registry` (`lib/core/registry.go:5-32`) tracks opaque handles exposed through CGO so other languages can reference long-lived Go objects safely.

### 2.2 Identities & cryptography
- `security.NewPrivateID` (`lib/security/identity.go:36`) fuses a secp256k1 ECIES key and an Ed25519 signing key into a single PrivateID, while `PublicID()` (`lib/security/identity.go:73`) derives the public half.
- Symmetric helpers like `security.AESEncrypt`/`AESDecrypt` (`lib/security/aescrypt.go:28-57`) and streaming encryptors/decryptors (`lib/security/aescrypt.go:112-181`) implement file/content encryption.
- CGO exports such as `bao_ecEncrypt` (`lib/export.go:147-160`) and `bao_aesEncrypt`/`bao_aesDecrypt` (`lib/export.go:184-216`) bridge those primitives to Python/Dart/JS.

### 2.3 Storage abstraction
- `storage.Store` (`lib/storage/store.go:52-81`) defines the contract (ReadDir/Read/Write/Stat/Delete/ID/Close/Describe).
- `storage.Open` inspects the connection URL to pick a backend (`lib/storage/store_open_default.go:12-27`), backing S3 (`lib/storage/s3.go:1-120`), Azure Files (`lib/storage/azure.go:1-80`), SFTP, local file system, WebDAV, in-memory, etc.
- `storage.LoadTestURLs` (`lib/storage/store.go:83-104`) reads `credentials.yaml` to drive integration tests like `TestS3`/`TestLocal` (`lib/storage/storage_test.go:16-147`).

### 2.4 Database layer (`sqlx`)
- `sqlx.Open` (`lib/sqlx/db.go:34-65`) wraps the configured driver (SQLite by default), applies bundled DDL, and tracks prepared statements, schema versions, and a settings cache.
- Tests create ephemeral DBs via `sqlx.NewTestDB` (`lib/sqlx/db.go:92-103`).

### 2.5 Bao lifecycle & data plane
- `bao.Bao` aggregates identity, DB, and storage handles plus IO throttling and housekeeping state (`lib/bao/bao.go:19-105`).
- Creation wipes the backing store, stages config changes, and grants the creator admin rights (`lib/bao/create.go:19-82`). Opening reloads config and starts background reconciliation (`lib/bao/open.go:20-54`).
- Access control and blockchain state are modeled through `Group`/`Access` (`lib/bao/blockchain.go:32-74`) and `Change` payloads (`lib/bao/changes.go:12-93`). Blocks are signed and verified via blake2b + Ed25519 (`lib/bao/blockchain.go:99-179`).
- Writes are two-phase: `writeRecord` registers metadata in SQLite (`lib/bao/write.go:33-78`), then `writeFile` streams encrypted heads/bodies to the storage backend while honoring IO throttling (`lib/bao/write.go:80-164`).
- Reads mirror the pattern: metadata lookup, flag updates, destination tracking, and decryption via AES or EC depending on the key id (`lib/bao/read.go:14-70`).
- `Sync` reconciles remote segments and ingests new files concurrently, guarded by `checkAndUpdateExternalChange` (`lib/bao/sync.go:17-118`).
- `housekeeping` periodically syncs blockchain/users/directories, drains pending IO, and enforces retention windows (`lib/bao/housekeeping.go:12-109`).
- The behaviors above are exercised in unit tests such as `TestBaoWrite` (async writes + reads, `lib/bao/write_test.go:9-82`) and `TestBaoSynchronize` (multi-user sync, `lib/bao/sync_test.go:9-60`).

### 2.6 SQL replication layer
- `bao_ql.BaoQLayer` binds a bao + group to a transactional log directory (`lib/bao_ql/bao_ql.go:13-45`).
- `Layer.Exec` (`lib/bao_ql/transaction.go:41-65`) buffers keyed SQL statements; `SyncTables` (`lib/bao_ql/transaction.go:67-107`) writes pending transactions to the bao, reads remote ones, and replays them under `queryLock` for deterministic ordering. `Query`/`Fetch`/`FetchOne` handle reads (`lib/bao_ql/query.go:8-72`).

### 2.7 Mailbox helper
- `mailbox.Message` encapsulates subject/body/attachments (`lib/mailbox/mailbox.go:16-37`).
- `Send` streams attachments + JSON metadata into a bao directory, and `Receive` reconstructs messages by filtering deleted files and unmarshalling attributes (`lib/mailbox/mailbox.go:23-81`). Tests in `lib/mailbox/mailbox_test.go` show expected flows.

## 3. Bindings & SDKs

### 3.1 Python (`bindings/py/`)
- `pbao` loads the correct shared library by inspecting platform/architecture (`bindings/py/pbao/baod.py:1-53`) and defines the `Result`/`Data` structs that mirror `C.Result`/`C.Data` from `lib/export.go`.
- Function signatures are declared in `bindings/py/pbao/baob.py:1-74`; they match the CGO exports, and `consume` (`bindings/py/pbao/baob.py:77-99`) converts returned JSON blobs or raw bytes while freeing memory via the Go-provided `free` symbol.
- The high-level API lives in `bindings/py/pbao/bao.py:75-200`, where `Bao.create/open` wrap `bao_create`/`bao_open`, `write_file` and `read_file` map to the Go IO pipeline, and helpers like `sqlLayer` or `send/receive` expose SQL replication and mailbox features.
- Tests/scripts can call `newPrivateID` (`bindings/py/pbao/bao.py:24-30`) to generate credentials and manipulate groups via `set_access` (`bindings/py/pbao/bao.py:115-122`).
- `bindings/py/build.sh:1-55` iterates over the compiled artifacts in `../build/*`, copies the right `.so/.dylib/.dll` into `pbao/_libs`, and runs `setup.py bdist_wheel` with platform-specific tags. Running `make py` ensures Go libraries exist before packaging.

### 3.2 Dart / Flutter (`bindings/dart/bao`)
- `initBaoLibrary` (see `bindings/dart/bao/lib/src/loader.dart:1-120`) locates platform-specific binaries (downloaded via `bindings/dart/install.sh:1-79` or produced locally) and exposes a `Result` object with automatic JSON decoding and error propagation based on the Go `wrappedError` payloads.
- `bindings.dart` (`bindings/dart/bao/lib/src/bindings.dart:1-120`) maps Dart method names to native symbols (e.g., `bao_setAccess`, `baoql_sync_tables`, `mailbox_receive`), performs argument marshaling (UTF-8 strings, JSON blobs, raw buffers), and keeps a list of allocated pointers to free after each call.
- The public API in `bindings/dart/bao/lib/src/bao.dart:17-120` mirrors the Go `Bao`: `Bao.create`/`open`, `setAccess`, `waitFiles`, `sync`, etc. Constants like `users`, `admins`, and `asyncOperation` match the Go enum values.
- `bindings/dart/bao/test/bao_test.dart:6-49` covers typical flows (creating a bao, granting access, writing a file, awaiting `waitFiles`), ensuring bindings stay in sync whenever the Go layer changes.
- Distribution scripts (`bindings/dart/install.sh`, `bindings/dart/publish.sh`) download the newest release assets, copy them into Flutter platform folders (`macos/Libraries`, `ios/Libraries`, etc.), and publish the package via `dart pub publish`.

### 3.3 Browser/WASM (`wasm/` + `lib/jsdemo/`)
- `lib/jsdemo/main_js.go:1-105` builds the Go library with `//go:build js`, registers promise-returning JS functions (`baoCreate`, `baoOpen`, `baoWrite`, `baoRead`, `baoList`), and demonstrates using the in-memory SQL engine for demo purposes.
- `wasm/shim.js:1-63` loads `wasm_exec.js`, instantiates the Go module, and forwards UI actions to the functions exported via `syscall/js`.
- `wasm/main.js:1-77` wires HTML buttons to the shim: bao create/open/write/read/list plus SQL DB calls for exec/fetch/fetchOne.
- Running `make wasm` produces `build/wasm/bao.wasm` and copies `wasm_exec.js`; `make web` starts a static server on `http://localhost:$WEB_PORT/wasm/index.html` for interactive testing.

## 4. Tooling & packaging

- **Cross-compilation:** `make lib` calls `mac ios windows linux android`, each of which calls `build_targets` (`Makefile:97-101`). `CC` is overridden with the correct cross toolchain, and `MODE=c-shared` produces shared libraries plus headers (`lib/cfunc.h`).
- **Language packages:** After `make lib`, run `make py` to build wheels (per architecture) or `make dart` to update the Flutter bindings with the new binaries.
- **XCFramework integration:** `build_xcframework` (`Makefile:66-80`) merges simulator and device archives, copies `module.modulemap:1-3`, and emits `build/ios/bao.xcframework` for Swift/Objective-C consumers.
- **Release automation:** `make release` bundles each platform directory into `bao_<platform>.zip`, creates an annotated git tag, pushes it, and uploads the zips to GitHub via `gh release create`.

## 5. Testing & validation

- **Go tests:** `make test` runs `go test ./...` inside `lib/`. Highlights:
  - `lib/bao/write_test.go:9-115` verifies staged writes, async reads, attribute propagation, and `WaitFiles` semantics.
  - `lib/bao/sync_test.go:9-60` ensures multi-user sync loads new files exactly once.
  - `lib/storage/storage_test.go:26-147` exercises the local backend (create dirs, write/read/delete files, list filters), and optionally S3/WebDAV when credentials are available.
- **Binding tests:** `bindings/dart/bao/test/bao_test.dart:6-49` runs in Flutter/Dart to validate the FFI surface. Python relies on integration notebooks/scripts in `bindings/py/jupyter` plus the same Go tests because Python is a thin wrapper around the CGO API.
- **WASM smoke tests:** Open `wasm/index.html` after `make web` to manually verify create/write/read/list plus DB commands via the UI.

## 6. Typical workflow

1. **Generate credentials:** Call `security.NewPrivateIDMust()` (`lib/security/identity.go:57`) or, from Python/Dart, `newPrivateID()` (`bindings/py/pbao/bao.py:24-30`) / `newPrivateID()` in Dart (see `bindings/dart/bao/lib/src/identity.dart`).
2. **Open a DB:** Use `sqlx.Open` or a binding equivalent (`bindings/py/pbao/bao.py:38-46` / `bindings/dart/bao/lib/src/db.dart`).
3. **Create a bao:** `bao.Create` (`lib/bao/create.go:19-82`) or `Bao.create` in Python/Dart, supplying the storage URL (e.g., `file:///…`, `s3://…`).
4. **Grant access:** `SyncAccess` (`lib/bao/access.go:14-180`) takes one or more `AccessChange` entries and stages the necessary blockchain records; bindings still expose `set_access` (`bindings/py/pbao/bao.py:115-122`) and `setAccess` (`bindings/dart/bao/lib/src/bao.dart:79-102`). Tests assert that ACLs stick (`bindings/dart/bao/test/bao_test.dart:12-27`).
5. **Write & read data:** `s.Write` (`lib/bao/write.go:170-198`) / `write_file` (`bindings/py/pbao/bao.py:158-164`) / `Bao.write` (`bindings/dart/bao/lib/src/bao.dart:138-157`), then `WaitFiles` and `Read`/`read_file`. The Go tests demonstrate verifying file metadata and contents.
6. **Sync & housekeeping:** Either rely on `startHousekeeping` (spawned in `Create`/`Open`) or call `Bao.Sync` manually (`lib/bao/sync.go:17-43`). Python offers `read_dir`, `sync`, and `set_retention`, while Dart uses `waitFiles` and `sync`.
7. **Use advanced features:** Start a SQL layer via `bao_sqlLayer` (`lib/export.go:620+`, consumed by `bindings/py/pbao/bao.py:180-186`), or exchange messages via `bao_send`/`bao_receive` exported in `lib/mailbox/mailbox.go:23-81`.

## 7. Licenses & contributions

- Dual-licensed under AGPL-3.0 and a commercial license (`LICENSE:1-11`). Contributing implies dual licensing.
- Each binding (`bindings/py/`, `bindings/dart/`) includes its own license/README; ensure redistribution follows both AGPL requirements and platform store policies.

For deeper Go API details, refer to `lib/README.md`. For questions about bindings or build outputs, the scripts in `bindings/py/`, `bindings/dart/`, and `wasm/` provide the authoritative reference implementations.
