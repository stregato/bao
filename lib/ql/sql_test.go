package ql

import (
	_ "embed"
	"testing"

	"github.com/stregato/bao/lib/bao"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

//go:embed test.sql
var testDdl string

func TestExec(t *testing.T) {
	alice := security.NewPrivateIDMust()
	aliceID := alice.PublicIDMust()
	bob := security.NewPrivateIDMust()
	storeConfig := storage.LoadTestConfig(t, "test")

	db := sqlx.NewTestDB(t, "bao.db", "")
	dataDb := sqlx.NewTestDB(t, "vault2.db", testDdl)

	s, err := bao.Create(db, alice, storeConfig, bao.Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = s.SyncAccess(0,
		bao.AccessChange{Group: bao.Users, Access: bao.ReadWrite, UserId: alice.PublicIDMust()},
		bao.AccessChange{Group: bao.Users, Access: bao.ReadWrite, UserId: bob.PublicIDMust()},
	)
	core.TestErr(t, err, "cannot set access: %v")

	sl, err := SQL(s, bao.Users, dataDb)
	core.TestErr(t, err, "cannot open db: %v")

	_, err = sl.FetchOne("SELECT_TEST_DATA", sqlx.Args{})
	core.Assert(t, err != nil, "expected error on empty table: %v", err)

	_, err = sl.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "hello world", "cnt": 1, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	_, err = sl.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "second hello", "cnt": 2, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	rows, err := sl.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 2, "expected 2 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "hello world", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "second hello", "unexpected value in second row: %s", rows[1][0])

	updates, err := sl.SyncTables()
	core.TestErr(t, err, "cannot commit: %v")
	core.Assert(t, updates == 2, "expected 0 updates, got %d", updates)
	rows, err = sl.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 2, "expected 2 rows, got %d", len(rows))

	_, err = sl.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "third hello", "cnt": 2, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err = sl.SyncTables()
	core.TestErr(t, err, "cannot commit: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	rows, err = sl.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 3, "expected 3 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "hello world", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "second hello", "unexpected value in second row: %s", rows[1][0])
	core.Assert(t, rows[2][0] == "third hello", "unexpected value in third row: %s", rows[2][0])

	s.WaitFiles() // Ensure all changes are written to the database

	s.Close()
	db.Close()
	dataDb.Close()

	db2 := sqlx.NewTestDB(t, "vault3.db", "")
	dataDb2 := sqlx.NewTestDB(t, "vault4.db", testDdl)

	s, err = bao.Open(db2, bob, storeConfig, aliceID)
	core.TestErr(t, err, "cannot open vault: %v")

	sl, err = SQL(s, bao.Users, dataDb2)
	core.TestErr(t, err, "cannot open db: %v")

	updates, err = sl.SyncTables()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 3, "expected 3 updates, got %d", updates)

	rows, err = sl.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 3, "expected 3 rows, got %d", len(rows))

	updates, err = sl.SyncTables()
	core.TestErr(t, err, "cannot commit: %v")
	core.Assert(t, updates == 0, "expected 0 updates, got %d", updates)

	rows, err = sl.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 3, "expected 3 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "hello world", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "second hello", "unexpected value in second row: %s", rows[1][0])
	core.Assert(t, rows[2][0] == "third hello", "unexpected value in third row: %s", rows[2][0])

	s.Close()
	db2.Close()
	dataDb2.Close()
}
