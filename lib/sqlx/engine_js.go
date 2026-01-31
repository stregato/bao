//go:build js

package sqlx

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"syscall/js"

	"github.com/stregato/bao/lib/core"
)

/*** Engine / transport ***/

type Engine struct {
	w       js.Value
	mu      sync.Mutex
	nextID  int
	pending map[int]chan js.Value
	onmsg   js.Func
}

type Stmt struct {
	eng    *Engine
	stmtID int
}

type Tx struct {
	eng  *Engine
	txID int
}

// Row/Rows implemented in-memory for predictable Scan.
type Row struct {
	cols []string
	decl []string
	data []any
	err  error
}

type Rows struct {
	cols []string
	decl []string
	data [][]any
	i    int
}

/*** Result ***/

type R struct{ v js.Value }
type Result = *R

func (r Result) LastInsertId() (int64, error) {
	if !r.v.Truthy() {
		return 0, errors.New("no result")
	}
	return int64(r.v.Get("lastInsertId").Int()), nil
}
func (r Result) RowsAffected() (int64, error) {
	if !r.v.Truthy() {
		return 0, errors.New("no result")
	}
	return int64(r.v.Get("rowsAffected").Int()), nil
}

/*** Open/Close ***/

func OpenEngine(driverName, dataSourceName string) (*Engine, error) {
	ctor := js.Global().Get("Worker")
	if !ctor.Truthy() {
		return nil, core.Errorw(core.GenericError, "Worker API not available")
	}
	w := ctor.New("./db_worker.js")

	e := &Engine{
		w:       w,
		nextID:  1,
		pending: make(map[int]chan js.Value),
	}
	e.onmsg = js.FuncOf(func(this js.Value, args []js.Value) any {
		d := args[0].Get("data")
		if !d.Truthy() {
			return nil
		}
		reqID := d.Get("reqId")
		if !reqID.Truthy() {
			return nil
		}
		id := reqID.Int()
		e.mu.Lock()
		ch, ok := e.pending[id]
		if ok {
			delete(e.pending, id)
		}
		e.mu.Unlock()
		if ok {
			ch <- d
		}
		return nil
	})
	e.w.Set("onmessage", e.onmsg)

	if _, err := e.call(context.Background(), "open", map[string]any{"path": dataSourceName}); err != nil {
		e.dispose()
		return nil, core.Errorw(core.DbError, "cannot open JS DB: %s", err.Error())
	}
	return e, nil
}

func (e *Engine) Close() error {
	_, err := e.call(context.Background(), "close", nil)
	e.dispose()
	return err
}

/*** Statement lifecycle ***/

func (e *Engine) Prepare(sql string) (*Stmt, error) {
	v, err := e.call(context.Background(), "prepare", map[string]any{"sql": sql})
	if err != nil {
		return nil, err
	}
	id := v.Get("stmtId").Int()
	if id == 0 {
		return nil, errors.New("prepare: missing stmtId")
	}
	return &Stmt{eng: e, stmtID: id}, nil
}

func (s *Stmt) Close() error {
	_, err := s.eng.call(context.Background(), "closeStmt", map[string]any{"stmtId": s.stmtID})
	return err
}

/*** Exec / Query (Engine-level, convenience) ***/

func (e *Engine) Exec(query string, args ...any) (Result, error) {
	v, err := e.call(context.Background(), "exec", map[string]any{
		"sql": query, "args": coerceArgs(args),
	})
	if err != nil {
		return nil, err
	}
	return &R{v: v.Get("result")}, nil
}

func (e *Engine) QueryRow(query string, args ...any) *Row {
	v, err := e.call(context.Background(), "queryRow", map[string]any{
		"sql": query, "args": coerceArgs(args),
	})
	if err != nil {
		return &Row{err: err}
	}
	cols := toStringSlice(v.Get("columns"))
	decl := toStringSlice(v.Get("declTypes"))
	if !v.Get("hasRow").Truthy() {
		return &Row{cols: cols, decl: decl, data: nil, err: errors.New("sql: no rows")}
	}
	return &Row{cols: cols, decl: decl, data: toAnySlice(v.Get("row"))}
}

func (e *Engine) Query(query string, args ...any) (*Rows, error) {
	v, err := e.call(context.Background(), "query", map[string]any{
		"sql": query, "args": coerceArgs(args),
	})
	if err != nil {
		return nil, err
	}
	rs := &Rows{
		cols: toStringSlice(v.Get("columns")),
		decl: toStringSlice(v.Get("declTypes")),
		data: to2DAny(v.Get("rows")),
		i:    0,
	}
	return rs, nil
}

/*** Exec / Query (Stmt-level; matches database/sql signatures) ***/

func (s *Stmt) Exec(args ...any) (Result, error) {
	v, err := s.eng.call(context.Background(), "execStmt", map[string]any{
		"stmtId": s.stmtID, "args": coerceArgs(args),
	})
	if err != nil {
		return nil, err
	}
	return &R{v: v.Get("result")}, nil
}

func (s *Stmt) Query(args ...any) (*Rows, error) {
	v, err := s.eng.call(context.Background(), "queryStmt", map[string]any{
		"stmtId": s.stmtID, "args": coerceArgs(args),
	})
	if err != nil {
		return nil, err
	}
	rs := &Rows{
		cols: toStringSlice(v.Get("columns")),
		decl: toStringSlice(v.Get("declTypes")),
		data: to2DAny(v.Get("rows")),
		i:    0,
	}
	return rs, nil
}

func (s *Stmt) QueryRow(args ...any) *Row {
	v, err := s.eng.call(context.Background(), "queryRowStmt", map[string]any{
		"stmtId": s.stmtID, "args": coerceArgs(args),
	})
	if err != nil {
		return &Row{err: err}
	}
	cols := toStringSlice(v.Get("columns"))
	decl := toStringSlice(v.Get("declTypes"))
	if !v.Get("hasRow").Truthy() {
		return &Row{cols: cols, decl: decl, err: errors.New("sql: no rows")}
	}
	return &Row{cols: cols, decl: decl, data: toAnySlice(v.Get("row"))}
}

/*** Row/Rows API (database/sql-like) ***/

func (r *Row) Err() error {
	if r == nil {
		return errors.New("nil Row")
	}
	return r.err
}

func (r *Row) Scan(dest ...any) error {
	if r == nil {
		return errors.New("nil Row")
	}
	if r.err != nil { // deferred error from QueryRow
		return r.err
	}
	if r.data == nil {
		return errors.New("no row")
	}
	if err := scanInto(dest, r.data); err != nil {
		// mirror database/sql behavior: keep the error available via Err().
		r.err = err
		return err
	}
	return nil
}

func (rs *Rows) Next() bool {
	if rs == nil {
		return false
	}
	if rs.i >= len(rs.data) {
		return false
	}
	rs.i++
	return true
}

func (rs *Rows) Scan(dest ...any) error {
	if rs.i == 0 || rs.i > len(rs.data) {
		return errors.New("no current row")
	}
	return scanInto(dest, rs.data[rs.i-1])
}

func (rs *Rows) Close() error { return nil }

type ColumnType struct {
	name   string
	dbType string // declared type (uppercase), may be "" for expressions
}

func (c ColumnType) Name() string             { return c.name }
func (c ColumnType) DatabaseTypeName() string { return c.dbType }

// Optional stubs to mirror database/sql.ColumnType
func (c ColumnType) Length() (int64, bool)              { return 0, false }
func (c ColumnType) DecimalSize() (p, s int64, ok bool) { return 0, 0, false }
func (c ColumnType) Nullable() (bool, bool)             { return false, false }
func (c ColumnType) ScanType() reflect.Type             { return reflect.TypeOf(new(any)).Elem() }

func (rs *Rows) ColumnTypes() ([]*ColumnType, error) {
	n := len(rs.cols)
	out := make([]*ColumnType, n)
	for i := 0; i < n; i++ {
		dbt := ""
		if i < len(rs.decl) {
			dbt = rs.decl[i]
		}
		ct := &ColumnType{name: rs.cols[i], dbType: dbt}
		out[i] = ct
	}
	return out, nil
}

/*** Transactions ***/

func (e *Engine) Begin() (*Tx, error) {
	v, err := e.call(context.Background(), "begin", nil)
	if err != nil {
		return nil, err
	}
	id := v.Get("txId").Int()
	if id == 0 {
		id = 1
	} // single-conn; ID not strictly needed
	return &Tx{eng: e, txID: id}, nil
}
func (t *Tx) Commit() error {
	_, err := t.eng.call(context.Background(), "commit", map[string]any{"txId": t.txID})
	return err
}
func (t *Tx) Rollback() error {
	_, err := t.eng.call(context.Background(), "rollback", map[string]any{"txId": t.txID})
	return err
}
func (t *Tx) Prepare(sql string) (*Stmt, error) {
	v, err := t.eng.call(context.Background(), "prepareTx", map[string]any{"txId": t.txID, "sql": sql})
	if err != nil {
		return nil, err
	}
	id := v.Get("stmtId").Int()
	if id == 0 {
		return nil, errors.New("prepareTx: missing stmtId")
	}
	return &Stmt{eng: t.eng, stmtID: id}, nil
}

/*** internals ***/

func (e *Engine) dispose() {
	if e.onmsg.Truthy() {
		e.onmsg.Release()
	}
	defer func() { _ = recover() }()
	if e.w.Truthy() {
		e.w.Call("terminate")
	}
}

func (e *Engine) call(ctx context.Context, op string, payload map[string]any) (js.Value, error) {
	e.mu.Lock()
	id := e.nextID
	e.nextID++
	ch := make(chan js.Value, 1)
	e.pending[id] = ch
	e.mu.Unlock()

	if payload == nil {
		payload = map[string]any{}
	}
	payload["op"] = op
	payload["reqId"] = id
	e.w.Call("postMessage", payload)

	select {
	case v := <-ch:
		if !v.Get("ok").Truthy() {
			return js.Undefined(), errors.New(v.Get("err").String())
		}
		return v, nil
	case <-ctx.Done():
		e.mu.Lock()
		delete(e.pending, id)
		e.mu.Unlock()
		return js.Undefined(), fmt.Errorf("%s canceled: %w", op, ctx.Err())
	}
}

func coerceArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		switch t := a.(type) {
		case []byte:
			out[i] = t // structured clone copies to ArrayBuffer
		default:
			out[i] = a
		}
	}
	return out
}

func toStringSlice(v js.Value) []string {
	if !v.Truthy() {
		return nil
	}
	n := v.Length()
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = v.Index(i).String()
	}
	return out
}
func toAnySlice(v js.Value) []any {
	if !v.Truthy() {
		return nil
	}
	n := v.Length()
	out := make([]any, n)
	for i := 0; i < n; i++ {
		el := v.Index(i)
		switch el.Type() {
		case js.TypeBoolean:
			out[i] = el.Bool()
		case js.TypeNumber:
			out[i] = el.Float()
		case js.TypeString:
			out[i] = el.String()
		case js.TypeObject:
			if el.InstanceOf(js.Global().Get("Uint8Array")) {
				b := make([]byte, el.Get("length").Int())
				js.CopyBytesToGo(b, el)
				out[i] = b
			} else {
				out[i] = el
			}
		default:
			out[i] = nil
		}
	}
	return out
}
func to2DAny(v js.Value) [][]any {
	if !v.Truthy() {
		return nil
	}
	n := v.Length()
	out := make([][]any, n)
	for i := 0; i < n; i++ {
		out[i] = toAnySlice(v.Index(i))
	}
	return out
}

func scanInto(dest []any, row []any) error {
	if len(dest) != len(row) {
		return fmt.Errorf("scan: want %d values, got %d", len(dest), len(row))
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *int:
			switch v := row[i].(type) {
			case int:
				*d = v
			case int64:
				*d = int(v)
			case float64:
				*d = int(v)
			default:
				return fmt.Errorf("scan int: %T", row[i])
			}
		case *int64:
			switch v := row[i].(type) {
			case int64:
				*d = v
			case int:
				*d = int64(v)
			case float64:
				*d = int64(v)
			default:
				return fmt.Errorf("scan int64: %T", row[i])
			}
		case *float64:
			switch v := row[i].(type) {
			case float64:
				*d = v
			case int:
				*d = float64(v)
			case int64:
				*d = float64(v)
			default:
				return fmt.Errorf("scan float64: %T", row[i])
			}
		case *string:
			switch v := row[i].(type) {
			case string:
				*d = v
			default:
				return fmt.Errorf("scan string: %T", row[i])
			}
		case *[]byte:
			switch v := row[i].(type) {
			case []byte:
				*d = v
			case string:
				*d = []byte(v)
			default:
				return fmt.Errorf("scan []byte: %T", row[i])
			}
		case *bool:
			switch v := row[i].(type) {
			case bool:
				*d = v
			case int:
				*d = v != 0
			case int64:
				*d = v != 0
			default:
				return fmt.Errorf("scan bool: %T", row[i])
			}
		case *any:
			*d = row[i]
		default:
			return fmt.Errorf("unsupported dest type %T", dest[i])
		}
	}
	return nil
}
