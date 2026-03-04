//go:build js

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf8"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/replica"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	libbao "github.com/stregato/bao/lib/vault"
)

var (
	demoDB        *sqlx.DB
	demoVault     *libbao.Vault
	demoStore     store.Store
	demoReplica   *replica.Replica
	demoReplicaDB *sqlx.DB
)

var validSQLIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func asPromise(fn func(this js.Value, args []js.Value) (any, error)) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		promiseConstructor := js.Global().Get("Promise")
		handler := js.FuncOf(func(_ js.Value, argv []js.Value) any {
			resolve := argv[0]
			reject := argv[1]
			go func() {
				defer func() {
					if r := recover(); r != nil {
						reject.Invoke(fmt.Sprintf("panic: %v\n%s", r, string(debug.Stack())))
					}
				}()
				if v, err := fn(this, args); err != nil {
					reject.Invoke(err.Error())
				} else {
					switch tv := v.(type) {
					case string:
						resolve.Invoke(tv)
					case []byte:
						resolve.Invoke(string(tv))
					default:
						b, _ := json.Marshal(tv)
						resolve.Invoke(string(b))
					}
				}
			}()
			return nil
		})
		return promiseConstructor.New(handler)
	})
}

func baoCreate(this js.Value, args []js.Value) (any, error) {
	var opts struct {
		PrivateID       string            `json:"privateId"`
		StoreURL        string            `json:"storeUrl"`
		StoreConfig     store.StoreConfig `json:"storageConfig"`
		StoreConfigJSON string            `json:"storageConfigJson"`
		DBPath          string            `json:"dbPath"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal([]byte(args[0].String()), &opts); err != nil {
			return nil, core.Error(core.ParseError, "invalid create options", err)
		}
	}
	if strings.TrimSpace(opts.DBPath) == "" {
		opts.DBPath = ":memory:"
	}

	if demoVault != nil {
		_ = demoVault.Close()
		demoVault = nil
	}
	if demoReplicaDB != nil {
		_ = demoReplicaDB.Close()
		demoReplicaDB = nil
	}
	demoReplica = nil
	if demoStore != nil {
		_ = demoStore.Close()
		demoStore = nil
	}
	if demoDB != nil {
		_ = demoDB.Close()
		demoDB = nil
	}

	var err error
	demoDB, err = openDBWithFallback(opts.DBPath)
	if err != nil {
		return nil, err
	}
	demoStore, err = openStoreFromOptions(opts.StoreURL, opts.StoreConfig, opts.StoreConfigJSON)
	if err != nil {
		return nil, err
	}
	id := security.PrivateID(strings.TrimSpace(opts.PrivateID))
	if id == "" {
		id = security.NewPrivateIDMust()
	}
	demoVault, err = libbao.Create(id, demoStore, demoDB, libbao.Config{})
	if err != nil {
		return nil, err
	}
	pub, _ := id.PublicID()
	return map[string]any{"ok": true, "publicId": pub, "store": demoStore.ID()}, nil
}

func baoOpen(this js.Value, args []js.Value) (any, error) {
	var opts struct {
		PrivateID       string            `json:"privateId"`
		StoreURL        string            `json:"storeUrl"`
		StoreConfig     store.StoreConfig `json:"storageConfig"`
		StoreConfigJSON string            `json:"storageConfigJson"`
		DBPath          string            `json:"dbPath"`
		Author          string            `json:"author"`
	}
	if len(args) == 0 {
		return nil, core.Error(core.ParseError, "missing open options")
	}
	if err := json.Unmarshal([]byte(args[0].String()), &opts); err != nil {
		return nil, core.Error(core.ParseError, "invalid open options", err)
	}
	if strings.TrimSpace(opts.PrivateID) == "" {
		return nil, core.Error(core.ParseError, "privateId is required")
	}
	if strings.TrimSpace(opts.DBPath) == "" {
		opts.DBPath = "myapp/main.sqlite"
	}

	if demoVault != nil {
		_ = demoVault.Close()
		demoVault = nil
	}
	if demoReplicaDB != nil {
		_ = demoReplicaDB.Close()
		demoReplicaDB = nil
	}
	demoReplica = nil
	if demoStore != nil {
		_ = demoStore.Close()
		demoStore = nil
	}
	if demoDB != nil {
		_ = demoDB.Close()
		demoDB = nil
	}

	var err error
	demoDB, err = openDBWithFallback(opts.DBPath)
	if err != nil {
		return nil, err
	}
	demoStore, err = openStoreFromOptions(opts.StoreURL, opts.StoreConfig, opts.StoreConfigJSON)
	if err != nil {
		return nil, err
	}

	id := security.PrivateID(strings.TrimSpace(opts.PrivateID))
	authorID := security.PublicID(strings.TrimSpace(opts.Author))
	if authorID == "" {
		authorID, _ = id.PublicID()
	}

	demoVault, err = libbao.Open(id, authorID, demoStore, demoDB)
	if err != nil {
		return nil, err
	}
	access, _ := demoVault.GetAccess(authorID)
	return map[string]any{"ok": true, "store": demoStore.ID(), "access": int(access)}, nil
}

func baoSync(this js.Value, args []js.Value) (any, error) {
	if demoVault == nil {
		return nil, core.Error(core.GenericError, "bao not opened")
	}
	newFiles, err := demoVault.Sync()
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "newFiles": len(newFiles)}, nil
}

func baoReadDir(this js.Value, args []js.Value) (any, error) {
	if demoVault == nil {
		return nil, core.Error(core.GenericError, "bao not opened")
	}
	var opts struct {
		Dir   string `json:"dir"`
		Limit int64  `json:"limit"`
	}
	if len(args) > 0 && strings.TrimSpace(args[0].String()) != "" {
		if err := json.Unmarshal([]byte(args[0].String()), &opts); err != nil {
			return nil, core.Error(core.ParseError, "invalid readDir options", err)
		}
	}
	if opts.Limit == 0 {
		opts.Limit = 200
	}
	ls, err := demoVault.ReadDir(opts.Dir, time.Time{}, 0, int(opts.Limit))
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(ls))
	for _, f := range ls {
		attrsText, attrsIsText := textAttrsPreview(f.Attrs)
		out = append(out, map[string]any{
			"id":      int64(f.Id),
			"name":    f.Name,
			"size":    f.Size,
			"modTime": f.ModTime,
			"isDir":   f.IsDir,
			"flags":   int64(f.Flags),
			"author":  f.AuthorId,
			"attrs": map[string]any{
				"present":   len(f.Attrs) > 0,
				"isText":    attrsIsText,
				"text":      attrsText,
				"size":      len(f.Attrs),
				"rawBase64": base64.StdEncoding.EncodeToString(f.Attrs),
			},
		})
	}
	return out, nil
}

func textAttrsPreview(b []byte) (string, bool) {
	if len(b) == 0 || !utf8.Valid(b) {
		return "", false
	}
	for _, r := range string(b) {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return "", false
		}
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", false
	}
	return s, true
}

func baoClose(this js.Value, args []js.Value) (any, error) {
	if demoVault != nil {
		_ = demoVault.Close()
		demoVault = nil
	}
	if demoReplicaDB != nil {
		_ = demoReplicaDB.Close()
		demoReplicaDB = nil
	}
	demoReplica = nil
	if demoStore != nil {
		_ = demoStore.Close()
		demoStore = nil
	}
	if demoDB != nil {
		_ = demoDB.Close()
		demoDB = nil
	}
	return map[string]any{"ok": true}, nil
}

func baoWrite(this js.Value, args []js.Value) (any, error) {
	if demoVault == nil {
		return nil, core.Error(core.GenericError, "bao not opened")
	}
	var opts struct {
		Path        string `json:"path"`
		DataBase64  string `json:"dataBase64"`
		AttrsBase64 string `json:"attrsBase64"`
		Async       bool   `json:"async"`
		Scheduled   bool   `json:"scheduled"`
	}
	if len(args) == 0 {
		return nil, core.Error(core.ParseError, "missing write options")
	}
	raw := strings.TrimSpace(args[0].String())
	if raw == "" {
		return nil, core.Error(core.ParseError, "empty write options")
	}
	if strings.HasPrefix(raw, "{") {
		if err := json.Unmarshal([]byte(raw), &opts); err != nil {
			return nil, core.Error(core.ParseError, "invalid write options", err)
		}
	} else {
		opts.Path = raw
	}
	opts.Path = strings.TrimSpace(opts.Path)
	if opts.Path == "" {
		return nil, core.Error(core.ParseError, "path is required")
	}
	if strings.TrimSpace(opts.DataBase64) == "" {
		return nil, core.Error(core.ParseError, "dataBase64 is required")
	}
	if opts.Async || opts.Scheduled {
		return nil, core.Error(core.NotImplemented, "async/scheduled upload not supported in wasm demo")
	}

	data, err := base64.StdEncoding.DecodeString(opts.DataBase64)
	if err != nil {
		return nil, core.Error(core.ParseError, "invalid dataBase64", err)
	}
	_ = data
	var attrs []byte
	if strings.TrimSpace(opts.AttrsBase64) != "" {
		attrs, err = base64.StdEncoding.DecodeString(opts.AttrsBase64)
		if err != nil {
			return nil, core.Error(core.ParseError, "invalid attrsBase64", err)
		}
	}

	source := "jsblob:" + opts.DataBase64
	file, err := demoVault.Write(opts.Path, source, attrs, libbao.IOOption{})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":      int64(file.Id),
		"name":    file.Name,
		"size":    file.Size,
		"modTime": file.ModTime,
		"ok":      true,
	}, nil
}

func baoRead(this js.Value, args []js.Value) (any, error) {
	if demoVault == nil {
		return nil, core.Error(core.GenericError, "bao not opened")
	}
	var opts struct {
		Path string `json:"path"`
	}
	if len(args) > 0 {
		raw := strings.TrimSpace(args[0].String())
		switch {
		case raw == "":
		case strings.HasPrefix(raw, "{"):
			if err := json.Unmarshal([]byte(raw), &opts); err != nil {
				return nil, core.Error(core.ParseError, "invalid read options", err)
			}
		default:
			opts.Path = raw
		}
	}
	opts.Path = strings.TrimSpace(opts.Path)
	if opts.Path == "" {
		return nil, core.Error(core.ParseError, "path is required")
	}

	tmpDest := fmt.Sprintf(".bao-wasm-read-%d.tmp", time.Now().UnixNano())
	file, err := demoVault.Read(opts.Path, tmpDest, libbao.IOOption{}, nil)
	if err != nil {
		return nil, err
	}
	defer libbao.RemoveLocalCopy(tmpDest)
	data, err := libbao.ReadLocalCopyBytes(tmpDest)
	if err != nil {
		return nil, core.Error(core.FileError, "cannot read downloaded local copy %s", tmpDest, err)
	}
	return map[string]any{
		"name":       filepath.Base(file.Name),
		"path":       opts.Path,
		"size":       len(data),
		"dataBase64": base64.StdEncoding.EncodeToString(data),
	}, nil
}

func baoList(this js.Value, args []js.Value) (any, error) {
	return baoReadDir(this, args)
}

func register() {
	js.Global().Set("baoNewPrivateID", asPromise(baoNewPrivateID))
	js.Global().Set("baoPublicID", asPromise(baoPublicID))
	js.Global().Set("baoCreate", asPromise(baoCreate))
	js.Global().Set("baoOpen", asPromise(baoOpen))
	js.Global().Set("baoSync", asPromise(baoSync))
	js.Global().Set("baoReadDir", asPromise(baoReadDir))
	js.Global().Set("baoClose", asPromise(baoClose))
	js.Global().Set("baoWrite", asPromise(baoWrite))
	js.Global().Set("baoRead", asPromise(baoRead))
	js.Global().Set("baoList", asPromise(baoList))
	js.Global().Set("replicaOpen", asPromise(replicaOpen))
	js.Global().Set("replicaSync", asPromise(replicaSync))
	js.Global().Set("replicaFetch", asPromise(replicaFetch))
	js.Global().Set("replicaExec", asPromise(replicaExec))
	js.Global().Set("replicaTables", asPromise(replicaTables))
	js.Global().Set("replicaTablePreview", asPromise(replicaTablePreview))
	// DB functions
	js.Global().Set("dbOpen", asPromise(dbOpen))
	js.Global().Set("dbExec", asPromise(dbExec))
	js.Global().Set("dbFetch", asPromise(dbFetch))
	js.Global().Set("dbFetchOne", asPromise(dbFetchOne))
}

func replicaOpen(this js.Value, args []js.Value) (any, error) {
	if demoVault == nil {
		return nil, core.Error(core.GenericError, "vault not opened")
	}
	var opts struct {
		DBPath string `json:"dbPath"`
		Dir    string `json:"dir"`
		DDL    string `json:"ddl"`
	}
	if len(args) > 0 {
		raw := strings.TrimSpace(args[0].String())
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &opts); err != nil {
				return nil, core.Error(core.ParseError, "invalid replica open options", err)
			}
		}
	}
	if strings.TrimSpace(opts.DBPath) == "" {
		opts.DBPath = "myapp/replica.sqlite"
	}
	if strings.TrimSpace(opts.Dir) == "" {
		opts.Dir = "replica"
	}
	if strings.TrimSpace(opts.Dir) != "replica" {
		return nil, core.Error(core.NotImplemented, "custom replica dir is not supported yet; use 'replica'")
	}
	if demoReplicaDB != nil {
		_ = demoReplicaDB.Close()
		demoReplicaDB = nil
	}
	demoReplica = nil
	ddl := strings.TrimSpace(opts.DDL)
	var err error
	demoReplicaDB, err = sqlx.Open("sqlite3", opts.DBPath, ddl)
	if err != nil {
		return nil, err
	}
	demoReplica, err = replica.Open(demoVault, demoReplicaDB)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "dbPath": opts.DBPath, "dir": opts.Dir}, nil
}

func replicaSync(this js.Value, args []js.Value) (any, error) {
	if demoReplica == nil {
		return nil, core.Error(core.GenericError, "replica not opened")
	}
	n, err := demoReplica.Sync()
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "updates": n}, nil
}

func normalizeReplicaQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	if strings.HasPrefix(q, "SQL:") {
		return q
	}
	return "SQL:" + q
}

func replicaFetch(this js.Value, args []js.Value) (any, error) {
	if demoReplica == nil {
		return nil, core.Error(core.GenericError, "replica not opened")
	}
	var opts struct {
		Query string         `json:"query"`
		Args  map[string]any `json:"args"`
		Max   int            `json:"max"`
	}
	if len(args) > 0 {
		raw := strings.TrimSpace(args[0].String())
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &opts); err != nil {
				return nil, core.Error(core.ParseError, "invalid replica fetch options", err)
			}
		}
	}
	if opts.Max <= 0 {
		opts.Max = 100
	}
	rows, err := demoReplica.Fetch(normalizeReplicaQuery(opts.Query), sqlx.Args(opts.Args), opts.Max)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func replicaExec(this js.Value, args []js.Value) (any, error) {
	if demoReplica == nil {
		return nil, core.Error(core.GenericError, "replica not opened")
	}
	var opts struct {
		Query string         `json:"query"`
		Args  map[string]any `json:"args"`
	}
	if len(args) > 0 {
		raw := strings.TrimSpace(args[0].String())
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &opts); err != nil {
				return nil, core.Error(core.ParseError, "invalid replica exec options", err)
			}
		}
	}
	res, err := demoReplica.Exec(normalizeReplicaQuery(opts.Query), sqlx.Args(opts.Args))
	if err != nil {
		return nil, err
	}
	ra, _ := res.RowsAffected()
	li, _ := res.LastInsertId()
	return map[string]any{"ok": true, "rowsAffected": ra, "lastInsertId": li}, nil
}

func replicaTables(this js.Value, args []js.Value) (any, error) {
	if demoReplica == nil {
		return nil, core.Error(core.GenericError, "replica not opened")
	}
	rows, err := demoReplica.Fetch("SQL:SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name", sqlx.Args{}, 1000)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if len(r) == 0 || r[0] == nil {
			continue
		}
		out = append(out, fmt.Sprintf("%v", r[0]))
	}
	return out, nil
}

func replicaTablePreview(this js.Value, args []js.Value) (any, error) {
	if demoReplica == nil {
		return nil, core.Error(core.GenericError, "replica not opened")
	}
	var opts struct {
		Table string `json:"table"`
		Limit int    `json:"limit"`
	}
	if len(args) > 0 {
		raw := strings.TrimSpace(args[0].String())
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &opts); err != nil {
				return nil, core.Error(core.ParseError, "invalid replica table preview options", err)
			}
		}
	}
	opts.Table = strings.TrimSpace(opts.Table)
	if opts.Table == "" {
		return nil, core.Error(core.ParseError, "table is required")
	}
	if !validSQLIdent.MatchString(opts.Table) {
		return nil, core.Error(core.ParseError, "invalid table name")
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	q := fmt.Sprintf("SQL:SELECT * FROM \"%s\" LIMIT %d", opts.Table, opts.Limit)
	rows, err := demoReplica.Fetch(q, sqlx.Args{}, opts.Limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func baoNewPrivateID(this js.Value, args []js.Value) (any, error) {
	id, err := security.NewPrivateID()
	if err != nil {
		return nil, err
	}
	return map[string]any{"privateId": string(id)}, nil
}

func baoPublicID(this js.Value, args []js.Value) (any, error) {
	var opts struct {
		PrivateID string `json:"privateId"`
	}
	if len(args) == 0 {
		return nil, core.Error(core.ParseError, "missing options")
	}
	if err := json.Unmarshal([]byte(args[0].String()), &opts); err != nil {
		return nil, core.Error(core.ParseError, "invalid options", err)
	}
	privateID := security.PrivateID(strings.TrimSpace(opts.PrivateID))
	if privateID == "" {
		return nil, core.Error(core.ParseError, "privateId is required")
	}
	publicID, err := privateID.PublicID()
	if err != nil {
		return nil, err
	}
	return map[string]any{"publicId": string(publicID)}, nil
}

func openStoreFromOptions(storeURL string, cfg store.StoreConfig, cfgJSON string) (store.Store, error) {
	if strings.TrimSpace(cfgJSON) != "" {
		var parsed store.StoreConfig
		if err := json.Unmarshal([]byte(cfgJSON), &parsed); err != nil {
			return nil, core.Error(core.ParseError, "invalid storageConfigJson", err)
		}
		cfg = parsed
	}
	if strings.TrimSpace(cfg.Type) != "" {
		if strings.TrimSpace(cfg.Id) == "" {
			cfg.Id = cfg.Type
		}
		return store.OpenWithConfig(cfg)
	}
	if strings.TrimSpace(storeURL) == "" {
		storeURL = "mem://demo"
	}
	return store.Open(storeURL)
}

func openDBWithFallback(preferredPath string) (*sqlx.DB, error) {
	paths := []string{}
	seen := map[string]bool{}
	push := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}

	push(preferredPath)
	push("myapp/main.sqlite")
	push(sqlx.MemoryDB)

	var errs []string
	for _, p := range paths {
		db, err := sqlx.Open("sqlite3", p, "")
		if err == nil {
			core.Info("WASM DB opened at path: %s", p)
			return db, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", p, err))
	}
	return nil, core.Error(core.DbError, "cannot open sqlite db with fallback chain: %s", strings.Join(errs, " | "))
}

func main() {
	register()
	select {}
}

// ---- DB helpers exposed to JS ----

func dbOpen(this js.Value, args []js.Value) (any, error) {
	driver := args[0].String()
	dbPath := args[1].String()
	var err error
	demoDB, err = sqlx.Open(driver, dbPath, "")
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "path": dbPath}, nil
}

func dbExec(this js.Value, args []js.Value) (any, error) {
	if demoDB == nil {
		return nil, core.Error(core.DbError, "db not opened")
	}
	key := args[0].String()
	var m map[string]any
	if len(args) > 1 {
		if err := json.Unmarshal([]byte(args[1].String()), &m); err != nil {
			return nil, err
		}
	} else {
		m = map[string]any{}
	}
	res, err := demoDB.Exec(key, sqlx.Args(m))
	if err != nil {
		return nil, err
	}
	ra, _ := res.RowsAffected()
	li, _ := res.LastInsertId()
	return map[string]any{"rowsAffected": ra, "lastInsertId": li}, nil
}

func dbFetch(this js.Value, args []js.Value) (any, error) {
	if demoDB == nil {
		return nil, core.Error(core.DbError, "db not opened")
	}
	key := args[0].String()
	var m map[string]any
	if err := json.Unmarshal([]byte(args[1].String()), &m); err != nil {
		return nil, err
	}
	max := 100
	if len(args) > 2 {
		max = args[2].Int()
	}
	rows, err := demoDB.Fetch(key, sqlx.Args(m), max)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func dbFetchOne(this js.Value, args []js.Value) (any, error) {
	if demoDB == nil {
		return nil, core.Error(core.DbError, "db not opened")
	}
	key := args[0].String()
	var m map[string]any
	if len(args) > 1 {
		if err := json.Unmarshal([]byte(args[1].String()), &m); err != nil {
			return nil, err
		}
	} else {
		m = map[string]any{}
	}
	row, err := demoDB.FetchOne(key, sqlx.Args(m))
	if err != nil {
		return nil, err
	}
	return row, nil
}
