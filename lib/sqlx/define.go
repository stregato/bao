package sqlx

import (
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"github.com/stregato/bao/lib/core"
)

func (db *DB) ensureVersionsTable() (map[float64]bool, error) {
	// Check if the versions table exists by querying sqlite_master
	var exists bool
	err := db.Engine.QueryRow(`SELECT 1 FROM sqlite_master WHERE type='table' AND name='versions'`).Scan(&exists)
	if err == sql.ErrNoRows {
		_, err = db.Engine.Exec(`CREATE TABLE versions (version REAL PRIMARY KEY)`)
		if err != nil {
			return nil, core.Error(core.DbError, "cannot create versions table", err)
		}
	} else if err != nil && err != sql.ErrNoRows {
		return nil, core.Error(core.DbError, "cannot check versions table", err)
	}
	rows, err := db.Engine.Query(`SELECT version FROM versions`)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot query versions table", err)
	}
	defer rows.Close()
	versions := make(map[float64]bool)
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err == nil {
			versions[v] = true
		}
	}
	return versions, nil
}

func (db *DB) Define(ddl string) error {
	core.Trace("Defining SQL statements from DDL...")
	importedVersions, err := db.ensureVersionsTable()
	if err != nil {
		return err
	}

	parts := strings.Split(ddl, "\n")

	var header string
	var version float64
	toInsertVersions := make(map[float64]bool)
	for i := 0; i < len(parts); i++ {
		part := strings.Trim(parts[i], " ")
		if len(part) == 0 {
			continue
		}
		if strings.HasPrefix(part, "-- ") {
			comment := strings.Trim(part[3:], " ")
			split := strings.SplitN(comment, " ", 2)
			header = split[0]
			version = 1.0 // default version
			if len(split) > 1 {
				vstr := strings.TrimSpace(split[1])
				if v, err := strconv.ParseFloat(vstr, 64); err == nil {
					version = v
				}
			}
		} else {
			var query string
			line := i
			for ; i < len(parts); i++ {
				part := strings.Trim(parts[i], " ")
				if len(part) == 0 {
					break
				}
				query += part + "\n"
			}
			if header == "INIT" {
				if !importedVersions[version] {
					err := db.initStatement(query, line)
					if err != nil {
						return err
					}
					toInsertVersions[version] = true
				}
			} else {
				// For query statements, keep the statement with the highest version
				prev, ok := db.versions[header]
				prevF := float64(prev)
				if !ok || version > prevF {
					db.versions[header] = float32(version)
					err := db.queryStatement(header, query, line)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// Insert all new versions at the end
	if len(toInsertVersions) > 0 {
		tx, err := db.Engine.Begin()
		if err == nil {
			stmt, err := tx.Prepare(`INSERT OR IGNORE INTO versions (version) VALUES (?)`)
			if err == nil {
				for v := range toInsertVersions {
					_, _ = stmt.Exec(v)
				}
				stmt.Close()
			}
			tx.Commit()
		}
	}

	core.Trace("SQL statements defined successfully\n")
	return nil
}

func (db *DB) initStatement(query string, line int) error {
	_, err := db.Engine.Exec(query)
	if err != nil {
		return core.Error(core.DbError, "cannot execute SQL Init stmt (line %d) '%s'\n", line, query, err)
	}
	firstLine := strings.Split(query, "\n")[0]
	core.Trace("SQL Init (line %d) executed successfully: %s\n", line, firstLine)
	return nil
}

func (db *DB) queryStatement(header, query string, line int) error {
	header = strings.TrimSpace(header)
	db.queries[header] = query

	//firstLine := strings.Split(query, "\n")[0] + "..."
	core.Trace("SQL statement '%s' (line %d) defined: '%s'\n", header, line, query)
	return nil
}

func (db *DB) Keys() []string {
	var keys []string

	for k := range db.stmts {
		keys = append(keys, k)
	}
	return keys
}

// func (db *DB) prepareStatement(key, s string, line int) error {
// 	key = strings.Trim(key, " ")

// 	if !strings.Contains(s, "#") {
// 		stmt, err := db.Db.Prepare(s)
// 		if core.IsErr(err, "cannot compile SQL statement '%s' (%d) '%s': %v\n", key, line, s) {
// 			return err
// 		}
// 		// Do not cache the prepared statement here, just check it compiles.
// 		stmt.Close()
// 		core.Info("SQL statement compiled: '%s' (%d): %s\n", key, line, s)
// 	}

// 	db.queries[key] = s
// 	return nil
// }

func (db *DB) getQuery(sql string, args Args) (string, error) {
	core.Trace("key '%s'", sql)
	if v, ok := db.queries[sql]; ok {
		v = replaceArgs(v, args)
		core.Trace("found in cache\n")
		return v, nil
	}

	if strings.HasPrefix(sql, "SQL:") {
		sql = strings.TrimLeft(sql, "SQL:")
		core.Trace("SQL query retrieved directly")
		return sql, nil
	} else {
		return "", core.Error(core.DbError, "query not found '%s'", sql)
	}
}

func (db *DB) getStatement(sql string, args Args) (*Stmt, error) {
	core.Trace("getting SQL statement for key '%s' with args %v", sql, args)
	db.stmtsLock.Lock()
	defer db.stmtsLock.Unlock()
	if v, ok := db.stmts[sql]; ok {
		core.Trace("SQL statement found in cache: '%s'\n", sql)
		return v, nil
	}

	sql, err := db.getQuery(sql, args)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get SQL statement '%s'", sql, err)
	}

	if v, ok := db.stmts[sql]; ok {
		core.Trace("SQL statement found in cache: '%s'\n", sql)
		return v, nil
	}

	stmt, err := db.Engine.Prepare(sql)
	if err != nil {
		return nil, core.Error(core.DbError, "cannot compile SQL statement for key '%s'", sql, err)
	}
	db.stmts[sql] = stmt
	core.Trace("SQL statement compiled: '%s'\n", sql)
	return stmt, nil
}

func replaceArgs(s string, args Args) string {

	// Compile a regular expression that matches words starting with '#'
	re := regexp.MustCompile(`#\w+`)

	// Use the ReplaceAllStringFunc method to replace matches using a custom function
	result := re.ReplaceAllStringFunc(s, func(match string) string {
		// Look up the key in the map. If found, return its value; otherwise, return the match unchanged.
		if val, ok := args[match]; ok {
			if ss, ok := val.(string); ok {
				return ss
			}
		}
		return match
	})

	return result
}
