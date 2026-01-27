//go:build js

package main

import (
	"encoding/json"
	"math/rand"
	"syscall/js"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	libbao "github.com/stregato/bao/lib/vault"
)

var (
	demoDB    *sqlx.DB
	demoStash *libbao.Bao
)

func asPromise(fn func(this js.Value, args []js.Value) (any, error)) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		promiseConstructor := js.Global().Get("Promise")
		handler := js.FuncOf(func(_ js.Value, argv []js.Value) any {
			resolve := argv[0]
			reject := argv[1]
			go func() {
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
	url := args[0].String()
	var err error
	demoDB, err = sqlx.Open("mem", "", "")
	if err != nil {
		return nil, err
	}

	id := security.NewPrivateIDMust()
	demoStash, err = libbao.Create(demoDB, id, url, libbao.Config{})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func baoOpen(this js.Value, args []js.Value) (any, error) {
	url := args[0].String()
	var err error
	if demoDB == nil {
		demoDB, err = sqlx.Open("mem", "", "")
		if err != nil {
			return nil, err
		}
	}
	id := security.NewPrivateIDMust()
	demoStash, err = libbao.Open(demoDB, id, url, security.PublicID(""))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func baoWrite(this js.Value, args []js.Value) (any, error) {
	if demoStash == nil {
		return nil, core.Errorw("bao not opened")
	}
	name := args[0].String()
	group := libbao.Group(args[1].String())
	content := args[2].String()
	fi, err := demoStash.Write(name, "", group, []byte(content), 0, nil)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

func baoRead(this js.Value, args []js.Value) (any, error) {
	if demoStash == nil {
		return nil, core.Errorw("bao not opened")
	}
	name := args[0].String()
	dest := "/tmp/" + name + "-" + time.Now().Format("20060102150405") + "-" + string(rune('a'+rand.Intn(26)))
	file, err := demoStash.Read(name, dest, 0, nil)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func baoList(this js.Value, args []js.Value) (any, error) {
	if demoStash == nil {
		return nil, core.Errorw("bao not opened")
	}
	dir := args[0].String()
	ls, err := demoStash.ReadDir(dir, time.Time{}, 0, 0)
	if err != nil {
		return nil, err
	}
	return ls, nil
}

func register() {
	js.Global().Set("baoCreate", asPromise(baoCreate))
	js.Global().Set("baoOpen", asPromise(baoOpen))
	js.Global().Set("baoWrite", asPromise(baoWrite))
	js.Global().Set("baoRead", asPromise(baoRead))
	js.Global().Set("baoList", asPromise(baoList))
	// DB functions
	js.Global().Set("dbOpen", asPromise(dbOpen))
	js.Global().Set("dbExec", asPromise(dbExec))
	js.Global().Set("dbFetch", asPromise(dbFetch))
	js.Global().Set("dbFetchOne", asPromise(dbFetchOne))
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
		return nil, core.Errorw("db not opened")
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
		return nil, core.Errorw("db not opened")
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
		return nil, core.Errorw("db not opened")
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
