package sqlx

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
)

type TxX struct {
	tx      *Tx
	db      *DB // Reference to the parent DB to access queries and statements
	queries map[string]string
}

func (db *DB) Begin() (*TxX, error) {
	tx, err := db.Engine.Begin()
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot begin transaction", err)
	}
	core.Trace("successfully started transaction")
	return &TxX{
		tx: tx,
		db: db,
	}, nil
}

func (tx *TxX) Exec(key string, m Args) (Result, error) {
	args, err := convert(m)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot convert args for key %s", key, m, err)
	}

	stmt, err := tx.getStatement(key, m)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get statement for key %s", key, m, err)
	}

	res, err := stmt.Exec(args...)
	tx.trace(key, m, err)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot execute statement for key %s", key, m, err)
	}

	rowsAffected, _ := res.RowsAffected()
	core.Trace("successfully executed statement %s, rows affected: %d", key, rowsAffected)
	return res, nil
}

func (tx *TxX) QueryRow(key string, m Args, dest ...any) error {
	args, err := convert(m)
	if err != nil {
		return err
	}

	stmt, err := tx.getStatement(key, m)
	if err != nil {
		return err
	}

	row := stmt.QueryRow(args...)
	err = row.Err()
	tx.trace(key, m, err)
	if err != sql.ErrNoRows && err != nil {
		return core.Error(core.DbError, "cannot execute query", err)
	}

	err = scanRow(row, dest...)
	if err != nil {
		return core.Error(core.GenericError, "cannot scan row for key %s with args %v", key, m, err)
	}
	core.Trace("successfully executed query: %s", key)
	return nil
}

func (tx *TxX) Query(key string, m Args) (RowsX, error) {
	args, err := convert(m)
	if err != nil {
		return RowsX{}, err
	}
	stmt, err := tx.getStatement(key, m)
	if err != nil {
		return RowsX{}, err
	}

	rows, err := stmt.Query(args...)
	if err != nil {
		if err == sql.ErrNoRows {
			core.Info("no rows found for query: %s", key)
			return RowsX{}, nil
		}
		return RowsX{}, core.Error(core.DbError, "cannot execute query for key %s with args %v", key, m, err)
	}
	tx.trace(key, m, err)

	columnsType, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return RowsX{}, core.Error(core.DbError, "cannot get column types for query: %s", key, err)
	}

	core.Trace("successfully executed query: %s", key)
	return RowsX{rows: rows, columnTypes: columnsType}, err
}

func (tx *TxX) Commit() error {
	err := tx.tx.Commit()
	if err != nil {
		return core.Error(core.GenericError, "cannot commit transaction", err)
	}
	tx.db.stmtsLock.Lock()
	defer tx.db.stmtsLock.Unlock()
	for key, stmt := range tx.db.stmts {
		if stmt != nil {
			stmt.Close()
		}
		delete(tx.db.stmts, key)
	}
	core.Trace("successfully committed transaction")
	return nil
}

func (tx *TxX) Rollback() error {
	err := tx.tx.Rollback()
	if err != nil {
		return core.Error(core.GenericError, "cannot rollback transaction", err)
	}
	core.Trace("successfully rolled back transaction")
	return nil
}

func (tx *TxX) GetVersion(key string) float32 {
	return tx.db.versions[key]
}

func (tx *TxX) getStatement(sql string, args Args) (*Stmt, error) {
	if s, ok := tx.db.queries[sql]; ok {
		s = replaceArgs(s, args)

		stmt, err := tx.tx.Prepare(s)
		if err != nil {
			return nil, core.Error(core.DbError, "cannot compile SQL statement for key '%s'", sql, err)
		}
		core.Trace("SQL statement found in transaction cache: '%s'", sql)
		return stmt, nil
	} else if strings.HasPrefix(sql, "SQL:") {
		sql = strings.TrimLeft(sql, "SQL:")
	} else {
		return nil, core.Error(core.GenericError, "statement not found '%s'", sql)
	}

	stmt, err := tx.tx.Prepare(sql)
	if err != nil {
		return nil, core.Error(core.DbError, "invalid SQL statement '%s'", sql, err)
	}
	core.Trace("SQL statement compiled in transaction: '%s'", sql)
	return stmt, nil
}

func (tx *TxX) trace(key string, m Args, err error) {
	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		q := tx.queries[key]
		for k, v := range m {
			q = strings.ReplaceAll(q, ":"+k, fmt.Sprintf("%v", v))
		}
		logrus.Tracef("SQL: %s: %v", q, err)
	}
}
