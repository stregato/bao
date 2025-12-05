package sqlx

import (
	s "database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/vmihailenco/msgpack/v5"
)

type Args map[string]any

var ErrNoRows = s.ErrNoRows

func convert(m Args) ([]any, error) {
	var args []any

	for k, v := range m {
		if strings.HasPrefix(k, "#") {
			continue
		}

		var c any
		switch v := v.(type) {
		case time.Time:
			c = v.UnixNano()
		case string, []byte, int, int8, int16, int32, int64, uint16, uint32, uint64, uint8, float32, float64, bool:
			c = v
		default:
			if v != nil {
				kind := reflect.TypeOf(v).Kind()
				if kind == reflect.Struct {
					var err error
					c, err = msgpack.Marshal(v)
					if core.IsErr(err, "cannot marshal attribute %s=%v: %v", k, v, err) {
						return nil, err
					}
				} else {
					c = v
				}
			}
		}
		args = append(args, s.Named(k, c))
	}
	return args, nil
}

type RowsX struct {
	count       int
	rows        *Rows
	columnTypes []*ColumnType
}

func (db *DB) trace(key string, m Args, err error) {
	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		q := db.queries[key]
		for k, v := range m {
			q = strings.ReplaceAll(q, ":"+k, fmt.Sprintf("%v", v))
		}
		logrus.Tracef("SQL: %s: %v", q, err)
	}
}

// Exec executes a SQL statement identified by the key with the provided arguments.
// It converts the arguments to a slice of any type, retrieves the prepared statement for the query,
// and executes the query using the statement. The results are returned as a Result object.
// If an error occurs during the conversion of arguments or while executing the query,
// it returns an error with additional context.
// The key is used to identify the SQL statement, and the Args map contains the parameters for the query.
// The Args map can contain named parameters, which will be replaced in the SQL statement.
// The function returns a Result object that contains the result of the execution, such as the number of rows affected.
// If the query execution fails, it returns an error with additional context.
// The Args map can contain named parameters, which will be replaced in the SQL statement.
// The function returns a Result object that contains the result of the execution, such as the number of rows affected.
// If the query execution fails, it returns an error with additional context.
func (db *DB) Exec(key string, m Args) (Result, error) {
	args, err := convert(m)
	if err != nil {
		return nil, err
	}

	// stmt, err := db.getStatement(key, m)
	// if err != nil {
	// 	return nil, err
	// }
	sql, err := db.getQuery(key, m)
	if err != nil {
		return nil, core.Errorw("cannot get query for key %s", key, err)
	}

	res, err := db.Engine.Exec(sql, args...)
	//	res, err := stmt.Exec(args...)
	db.trace(key, m, err)
	if err != nil {
		return nil, core.Errorw("cannot execute query", err)
	}

	res.RowsAffected()
	return res, nil
}

// GetVersion retrieves the version associated with a given key from the database.
// It returns a float32 representing the version.
// If the key does not exist in the versions map, it returns 0.
// This function is useful for tracking the version of a specific query or operation in the database.
func (db *DB) GetVersion(key string) float32 {
	return db.versions[key]
}

// QueryRow executes a SQL query identified by the key with the provided arguments and fetches a single row.
// It converts the arguments to a slice of any type, retrieves the prepared statement for the query,
// and executes the query using the statement. The results are scanned into the provided destination variables.
// If an error occurs during the conversion of arguments or while executing the query,
// it returns an error with additional context.
// If the query returns no rows, it returns ErrNoRows.
// The destination variables must be pointers to the expected types.
func (db *DB) QueryRow(key string, m Args, dest ...any) error {
	args, err := convert(m)
	if err != nil {
		return err
	}

	stmt, err := db.getStatement(key, m)
	if err != nil {
		return err
	}

	row := stmt.QueryRow(args...)
	err = row.Err()
	db.trace(key, m, err)
	if err != s.ErrNoRows && err != nil {
		return core.Errorw("cannot execute query", err)
	}

	return scanRow(row, dest...)
}

// Query executes a SQL query identified by the key with the provided arguments.
// It converts the arguments to a slice of any type, retrieves the prepared statement for the query,
// and executes the query using the statement. The results are returned as a Rows object.
// If an error occurs during the conversion of arguments or while executing the query,
// it returns an error with additional context.
func (db *DB) Query(key string, m Args) (RowsX, error) {
	core.Trace("executing query %s with args %v", key, m)
	args, err := convert(m)
	if err != nil {
		return RowsX{}, core.Errorw("cannot convert args", err)
	}

	stmt, err := db.getStatement(key, m)
	if err != nil {
		return RowsX{}, core.Errorw("cannot get statement for query %s", key, err)
	}

	rows, err := stmt.Query(args...)
	if err != nil {
		return RowsX{}, core.Errorw("cannot execute query %s", key, err)
	}
	db.trace(key, m, err)

	columnsType, err := rows.ColumnTypes()
	if err != nil {
		rows.Close()
		return RowsX{}, core.Errorw("cannot get column types for query %s", key, err)
	}
	core.Trace("successfully query: %s, args %v", key, m)
	return RowsX{rows: rows, columnTypes: columnsType}, err
}

func (db *DB) Fetch(key string, m Args, max int) ([][]any, error) {
	rw, err := db.Query(key, m)
	if err != nil {
		return nil, core.Errorw("cannot execute query %s", key, err)
	}
	defer rw.Close()

	var results [][]any
	for i := 0; i < max && rw.Next(); i++ {
		row, err := rw.Current()
		if err != nil {
			return nil, core.Errorw("cannot fetch row %d for query %s", i, key, err)
		}
		results = append(results, row)
	}

	return results, nil
}

func (db *DB) FetchOne(key string, m Args) ([]any, error) {
	rw, err := db.Query(key, m)
	if err != nil {
		return nil, core.Errorw("cannot execute query %s", key, err)
	}
	defer rw.Close()

	if !rw.Next() {
		return nil, ErrNoRows
	}
	row, err := rw.Current()
	if err != nil {
		return nil, core.Errorw("cannot fetch one row for query %s", key, err)
	}

	return row, nil
}

// Map converts a struct to an Args map, using the struct's field names or "db" tags as keys.
// It skips fields with the "db" tag set to "-" and uses the field name if no "db" tag is present.
// The resulting Args map can be used for SQL queries or other purposes where a key-value representation is needed.
// It returns an empty Args map if the input is not a struct or if it has no fields.
// If the input is a pointer to a struct, it dereferences it to access the underlying struct.
// This function is useful for converting Go structs to a format suitable for database operations or other key-value based systems.
func Map(v any) Args {
	args := Args{}
	val := reflect.ValueOf(v)
	// Handle if the input is a pointer to a struct
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	// Ensure we're dealing with a struct
	if val.Kind() != reflect.Struct {
		return args
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		structField := typ.Field(i)
		// Use the db tag as the field name if present
		fieldName := structField.Tag.Get("db")
		// If the db tag is "-", exclude the field
		if fieldName == "-" {
			continue
		}
		if fieldName == "" {
			fieldName = structField.Name
		}
		args[fieldName] = field.Interface()
	}

	return args
}

// Scan scans the current row into the provided destination variables.
// It handles special cases for time.Time, bool, and binary data types.
// For time.Time, it converts Unix timestamps stored as INT to time.Time.
// For bool, it converts integer values to boolean.
// For binary data types (BLOB, TEXT), it unmarshals the data into the appropriate struct type using msgpack.
// It returns an error if the scan operation fails.
// The destination variables must be pointers to the expected types.
func (rw *RowsX) Scan(dest ...interface{}) (err error) {
	for i, col := range rw.columnTypes {
		switch col.DatabaseTypeName() {
		case "INTEGER", "INT": // Assuming the column is a Unix timestamp stored as INT
			if t, ok := dest[i].(*time.Time); ok {
				var timestamp int64
				dest[i] = &timestamp
				defer func(index int, originalDest *time.Time) {
					*originalDest = time.Unix(timestamp/1e9, timestamp%1e9)
				}(i, t)
			}
			if b, ok := dest[i].(*bool); ok {
				var boolean bool
				dest[i] = &boolean
				defer func(index int, originalDest *bool) {
					*originalDest = boolean
				}(i, b)
			}
		case "BLOB", "TEXT":
			if _, ok := dest[i].(*string); !ok {
				var kind = reflect.TypeOf(dest[i]).Elem().Kind()
				if kind == reflect.Struct {
					var data []byte
					var originalDest = dest[i]
					dest[i] = &data
					defer func(index int, originalDest any) {
						if len(data) > 0 {
							err = msgpack.Unmarshal(data, originalDest)
							if err != nil {
								n := reflect.TypeOf(originalDest)
								core.IsErr(err, "cannot convert binary to type %v: %v", n, err)
							}
						}
					}(i, originalDest)
				}
			}
		}
	}

	err = rw.rows.Scan(dest...)
	if err != nil {
		return core.Errorw("cannot scan row", err)
	}
	return nil
}

// scanRow scans the current row into the provided destination variables.
// It handles special cases for time.Time, bool, and binary data types.
// For time.Time, it converts Unix timestamps stored as INT to time.Time.
// For bool, it converts integer values to boolean.
// For binary data types (BLOB, TEXT), it unmarshals the data into the appropriate struct type using msgpack.
// It returns an error if the scan operation fails.
// The destination variables must be pointers to the expected types.
// This function is used internally by the Rows type to scan the current row.
// It is designed to be flexible and handle various data types, including time, strings, integers, and custom structs.
func scanRow(row *Row, dest ...interface{}) (err error) {
	for i, d := range dest {
		switch d := d.(type) {
		case *time.Time:
			var timestamp int64
			var originalDest = d
			dest[i] = &timestamp
			defer func(index int, originalDest *time.Time) {
				*originalDest = time.Unix(timestamp/1e9, timestamp%1e9)
			}(i, originalDest)
		case *string, *[]byte, *int, *int8, *int16, *int32, *int64, *uint16, *uint32, *uint64, *uint8, *float32, *float64, *bool:
			continue
		default:
			var kind = reflect.TypeOf(dest[i]).Elem().Kind()
			if kind == reflect.Struct {
				var data []byte
				var originalDest = dest[i]
				dest[i] = &data
				defer func(index int, originalDest any) {
					if len(data) > 0 {
						err = msgpack.Unmarshal(data, originalDest)
						if err != nil {
							n := reflect.TypeOf(originalDest)
							core.IsErr(err, "cannot convert param %d binary to type %v: %v", index, n, err)
						}
					}
				}(i, originalDest)
			}
		}
	}

	return row.Scan(dest...)
}

// Next advances the Rows object to the next row, increments the count of rows read, and logs the total count.
// It returns true if there is a next row, or false if there are no more rows to read.
func (rw *RowsX) Next() bool {
	if rw.rows == nil {
		return false
	}

	res := rw.rows.Next()
	if res {
		rw.count++
	} else {
		core.Trace("no more rows, %d total rows", rw.count)
	}

	return res
}

// Current retrieves the current row from the Rows object, scans it into a slice of interface{} values, and returns it.
// If the scan operation fails, it returns an error.
// The returned slice contains the values of the current row, with each value corresponding to a column in the result set.
// If there are no rows, it returns nil and an error indicating that there are no rows to read.
func (rw *RowsX) Current() ([]any, error) {
	if rw.rows == nil {
		return nil, core.Errorw("no rows to read")
	}

	values := make([]any, len(rw.columnTypes))
	valuePtrs := make([]any, len(rw.columnTypes))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	err := rw.Scan(valuePtrs...)
	if err != nil {
		return nil, core.Errorw("cannot scan current row", err)
	}

	core.Trace("values %v", values)
	return values, nil
}

func (rw *RowsX) Close() error {
	if rw.rows == nil {
		return nil
	}
	return rw.rows.Close()
}

// Fetch fetches up to max rows from the Rows object, returning them as a slice of slices of interface{}.
// It iterates through the rows, retrieves the current row using Current(), and appends it to the result slice.
// If an error occurs while fetching the current row, it returns an error with additional context.
// After fetching the rows, it closes the Rows object and clears the rows field to prevent further access.
// The returned slice contains the values of the fetched rows, with each row represented as a slice of interface{} values.
// If there are no rows to fetch, it returns nil and no error.
func (rw *RowsX) Fetch(max int) ([][]any, error) {
	if rw.rows == nil {
		return nil, nil
	}
	var rows [][]any
	for i := 0; i < max && rw.Next(); i++ {
		row, err := rw.Current()
		if err != nil {
			return nil, core.Errorw("cannot fetch current row", err)
		}
		rows = append(rows, row)
	}
	rw.Close()
	rw.rows = nil // Clear the rows to prevent further access

	core.Info("Fetched %d rows", len(rows))
	return rows, nil
}
