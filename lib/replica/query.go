package replica

import (
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

func (r *Replica) Query(query string, args sqlx.Args) (sqlx.RowsX, error) {
	core.Start("query %s, %d args, transaction %t", query, len(args), r.transaction != nil)
	r.queryLock.Lock()
	defer r.queryLock.Unlock()

	var err error
	var rows sqlx.RowsX

	if r.transaction != nil {
		rows, err = r.transaction.tx.Query(query, args)
		if err != nil {
			return sqlx.RowsX{}, core.Error(core.DbError, "cannot execute query %s in transaction", query, err)
		}
	} else {
		rows, err = r.db.Query(query, args)
		if err != nil {
			return sqlx.RowsX{}, core.Error(core.DbError, "cannot execute query %s", query, err)
		}
	}
	core.End("")
	return rows, nil
}

// Fetch executes a query and returns the results as a slice of slices of any.
// Each inner slice represents a row, and each element in the inner slice represents a column value.
// The max parameter limits the number of rows returned.
func (r *Replica) Fetch(query string, args sqlx.Args, max int) ([][]any, error) {
	core.Start("query %s, %d args, max %d", query, len(args), max)
	rows, err := r.Query(query, args)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot fetch query %s", query, err)
	}
	defer rows.Close()
	var results [][]any
	for i := 0; rows.Next() && i < max; i++ {
		var row []any
		row, err = rows.Current()
		if err != nil {
			return nil, core.Error(core.DbError, "cannot get current row", err)
		}
		results = append(results, row)
	}
	core.End("%d", len(results))
	return results, nil
}

// FetchOne executes a query and returns the first row as a slice of any.
func (r *Replica) FetchOne(query string, args sqlx.Args) ([]any, error) {
	core.Start("query %s, %d args", query, len(args))
	rows, err := r.Query(query, args)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot fetch one query %s", query, err)
	}
	defer rows.Close()
	if !rows.Next() {
		core.End("no rows")
		return nil, sqlx.ErrNoRows
	}
	row, err := rows.Current()
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get current row", err)
	}
	core.End("1 row")
	return row, nil
}
