package replica

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/stregato/bao/lib/vault"
)

//go:embed test.sql
var testDdl string

func TestExec(t *testing.T) {
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()

	db := sqlx.NewTestDB(t, "bao.db", "")
	dataDb := sqlx.NewTestDB(t, "vault2.db", testDdl)
	s, err := store.Open(store.LoadTestConfig(t, "test"))
	core.TestErr(t, err, "cannot open store: %v", err)
	defer s.Close()

	v, err := vault.Create(vault.Users, aliceSecret, s, db, vault.Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = v.SyncAccess(0,
		vault.AccessChange{Access: vault.ReadWrite, UserId: bob},
	)
	core.TestErr(t, err, "cannot set access: %v")

	replica, err := Open(v, dataDb)
	core.TestErr(t, err, "cannot open db: %v")

	_, err = replica.FetchOne("SELECT_TEST_DATA", sqlx.Args{})
	core.Assert(t, err != nil, "expected error on empty table: %v", err)

	_, err = replica.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "hello world", "cnt": 1, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	_, err = replica.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "second hello", "cnt": 2, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	rows, err := replica.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 2, "expected 2 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "hello world", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "second hello", "unexpected value in second row: %s", rows[1][0])

	updates, err := replica.Sync()
	core.TestErr(t, err, "cannot commit: %v")
	core.Assert(t, updates == 2, "expected 0 updates, got %d", updates)
	rows, err = replica.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 2, "expected 2 rows, got %d", len(rows))

	_, err = replica.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "third hello", "cnt": 2, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err = replica.Sync()
	core.TestErr(t, err, "cannot commit: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	rows, err = replica.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 3, "expected 3 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "hello world", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "second hello", "unexpected value in second row: %s", rows[1][0])
	core.Assert(t, rows[2][0] == "third hello", "unexpected value in third row: %s", rows[2][0])

	v.WaitFiles(context.Background()) // Ensure all changes are written to the database

	v.Close()
	db.Close()
	dataDb.Close()
	s.Close()

	db2 := sqlx.NewTestDB(t, "vault3.db", "")
	dataDb2 := sqlx.NewTestDB(t, "vault4.db", testDdl)
	s = store.LoadTestStore(t, "test")

	v, err = vault.Open(vault.Users, bobSecret, alice, s, db2)
	core.TestErr(t, err, "cannot open vault: %v")

	replica, err = Open(v, dataDb2)
	core.TestErr(t, err, "cannot open db: %v")

	updates, err = replica.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 3, "expected 3 updates, got %d", updates)

	rows, err = replica.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 3, "expected 3 rows, got %d", len(rows))

	updates, err = replica.Sync()
	core.TestErr(t, err, "cannot commit: %v")
	core.Assert(t, updates == 0, "expected 0 updates, got %d", updates)

	rows, err = replica.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 3, "expected 3 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "hello world", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "second hello", "unexpected value in second row: %s", rows[1][0])
	core.Assert(t, rows[2][0] == "third hello", "unexpected value in third row: %s", rows[2][0])

	v.Close()
	db2.Close()
	dataDb2.Close()
	s.Close()
}
