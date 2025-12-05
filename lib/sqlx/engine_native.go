//go:build !js

package sqlx

import (
	"database/sql"
	"errors"
	"os"
	"path"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
)

type Engine struct {
	DB *sql.DB
}

type Stmt = sql.Stmt

type Tx = sql.Tx
type Result = sql.Result
type Rows = sql.Rows
type Row = sql.Row
type ColumnType = sql.ColumnType

func OpenEngine(driverName, dataSourceName string) (*Engine, error) {
	core.Start("Opening database %s with driver %s", dataSourceName, driverName)
	if dataSourceName != MemoryDB && driverName == "sqlite3" {
		_, err := os.Stat(dataSourceName)
		if errors.Is(err, os.ErrNotExist) {
			logrus.Debugf("Database file does not exist, creating new file at: %s", dataSourceName)
			err := os.WriteFile(dataSourceName, []byte{}, 0644)
			if err != nil {
				return nil, core.Errorw("Cannot create SQLite db file %s", dataSourceName, err)
			}
		} else if err != nil {
			return nil, core.Errorw("Cannot access SQLite db file %s", dataSourceName, err)
		}
	}

	dir := path.Dir(dataSourceName)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, core.Errorw("Cannot create directory %s for SQLite db %s", dir, dataSourceName, err)
	}

	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, core.Errorw("Cannot open SQLite db %s with driver %s", dataSourceName, driverName, err)
	}

	core.End("successfully opened SQLite db %s with driver %s", dataSourceName, driverName)
	return &Engine{DB: db}, nil
}

func (e *Engine) Close() error {
	return e.DB.Close()
}

func (e *Engine) Prepare(query string) (*Stmt, error) {
	stmt, err := e.DB.Prepare(query)
	if err != nil {
		return nil, core.Errorw("cannot prepare statement", err)
	}
	return stmt, nil
}

func (e *Engine) Exec(query string, args ...any) (Result, error) {
	return e.DB.Exec(query, args...)
}

func (e *Engine) QueryRow(query string, args ...any) *Row {
	return e.DB.QueryRow(query, args...)
}

func (e *Engine) Query(query string, args ...any) (*Rows, error) {
	return e.DB.Query(query, args...)
}

func (e *Engine) Begin() (*Tx, error) {
	tx, err := e.DB.Begin()
	if err != nil {
		return nil, core.Errorw("cannot begin transaction", err)
	}
	return (*Tx)(tx), nil
}
