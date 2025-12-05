package sqlx

import (
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/stregato/bao/lib/core"
)

type DB struct {
	DbPath string
	Engine *Engine

	queries      map[string]string
	stmts        map[string]*Stmt
	stmtsLock    sync.Mutex
	versions     map[string]float32
	settingCache map[string]struct {
		s string
		i int64
		f float64
		b []byte
	}
	settingCacheMu sync.RWMutex
}

var Default *DB

var MemoryDB = ":memory:"

func Open(driverName, dataSourceName, ddl string) (*DB, error) {
	core.Start("Opening SQLite db %s with driver %s", dataSourceName, driverName)
	engine, err := OpenEngine(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	d := &DB{
		DbPath: dataSourceName,
		Engine: engine,
		settingCache: make(map[string]struct {
			s string
			i int64
			f float64
			b []byte
		}),
		versions: map[string]float32{},
		queries:  map[string]string{},
		stmts:    map[string]*Stmt{},
	}

	ddl = strings.TrimSpace(ddl)
	if ddl != "" {
		err = d.Define(ddl)
		if err != nil {
			return nil, core.Errorw("Cannot define SQLite db in %s", dataSourceName, err)
		}
	}

	core.Info("successfully opened SQLite db %s with driver %s", dataSourceName, driverName)
	core.End("")
	return d, nil
}

func (db *DB) Close() error {
	core.Start("Closing SQLite db %s", db.DbPath)
	err := db.Engine.Close()
	if err != nil {
		return core.Errorw("Cannot close SQLite db %s", db.DbPath, err)
	}
	core.Info("successfully closed SQLite db %s", db.DbPath)
	core.End("")
	return nil
}

func (db *DB) Delete() error {
	core.Start("Deleting SQLite db file %s", db.DbPath)
	if db.DbPath != MemoryDB {
		err := os.Remove(db.DbPath)
		if err != nil {
			return core.Errorw("Cannot delete SQLite db file %s", db.DbPath, err)
		}
	}
	core.Info("successfully deleted SQLite db file %s", db.DbPath)
	core.End("")
	return nil
}

// func (db *DB) GetConnection() *sql.DB {
// 	core.Info("Getting connection for database at path: %s", db.DbPath)
// 	return db.Engine
// }

func NewTestDB(t *testing.T, name, ddl string) *DB {
	core.Start("Creating test database at path: %s", name)
	name = path.Join(os.TempDir(), name)
	os.Remove(name)
	//name = fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=OFF&_cache_size=10000&_temp_store=MEMORY", name)
	db, err := Open("sqlite3", name, ddl)
	if err != nil {
		panic(err)
	}
	core.TestErr(t, err, "invalid dll: %v")

	core.End("Created test database at path: %s", name)
	return db
}
