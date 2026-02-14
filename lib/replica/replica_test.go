package replica

import (
	"context"
	_ "embed"
	"testing"

	"github.com/sirupsen/logrus"
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

func TestExecAliceBobTogether(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()

	db := sqlx.NewTestDB(t, "bao.db", "")
	dataDb := sqlx.NewTestDB(t, "vault2.db", testDdl)
	s, err := store.Open(store.LoadTestConfig(t, "test"))
	core.TestErr(t, err, "cannot open store: %v", err)
	defer s.Close()

	// Create vault for Alice
	vAlice, err := vault.Create(vault.Users, aliceSecret, s, db, vault.Config{})
	core.TestErr(t, err, "Create failed: %v")

	// Grant Bob read-write access
	err = vAlice.SyncAccess(0,
		vault.AccessChange{Access: vault.ReadWrite, UserId: bob},
	)
	core.TestErr(t, err, "cannot set access: %v")

	// Bob opens vault and replica without closing Alice's
	db2 := sqlx.NewTestDB(t, "vault3.db", "")
	dataDb2 := sqlx.NewTestDB(t, "vault4.db", testDdl)
	s2 := store.LoadTestStore(t, "test")

	vBob, err := vault.Open(vault.Users, bobSecret, alice, s2, db2)
	core.TestErr(t, err, "cannot open vault: %v")

	replicaBob, err := Open(vBob, dataDb2)
	core.TestErr(t, err, "cannot open db: %v")

	// Alice opens replicaAlice
	replicaAlice, err := Open(vAlice, dataDb)
	core.TestErr(t, err, "cannot open db: %v")

	// Alice inserts data
	_, err = replicaAlice.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "alice data 1", "cnt": 1, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err := replicaAlice.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	// Bob syncs and sees Alice's data
	updates, err = replicaBob.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	rows, err := replicaBob.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 1, "expected 1 row, got %d", len(rows))
	core.Assert(t, rows[0][0] == "alice data 1", "unexpected value: %s", rows[0][0])

	// Bob inserts data (both vaults stay open)
	_, err = replicaBob.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "bob data 1", "cnt": 2, "ratio": 0.5, "bin": []byte{4, 5, 6}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err = replicaBob.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	// Alice syncs again to see Bob's data
	updates, err = replicaAlice.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	rows, err = replicaAlice.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 2, "expected 2 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "alice data 1", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "bob data 1", "unexpected value in second row: %s", rows[1][0])

	// Both insert more data
	_, err = replicaAlice.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "alice data 2", "cnt": 3, "ratio": 0.5, "bin": []byte{7, 8, 9}})
	core.TestErr(t, err, "cannot insert test data: %v")

	_, err = replicaBob.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "bob data 2", "cnt": 4, "ratio": 0.5, "bin": []byte{10, 11, 12}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err = replicaAlice.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	updates, err = replicaBob.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 2, "expected 2 updates, got %d", updates)

	updates, err = replicaAlice.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	// Verify both have all 4 records
	rows, err = replicaAlice.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 4, "expected 4 rows in Alice's replica, got %d", len(rows))

	rows, err = replicaBob.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 4, "expected 4 rows in Bob's replica, got %d", len(rows))

	vAlice.Close()
	vBob.Close()

	vAlice, err = vault.Open(vault.Users, aliceSecret, alice, s, db)
	core.TestErr(t, err, "cannot reopen Alice's vault: %v", err)

	vBob, err = vault.Open(vault.Users, bobSecret, alice, s2, db2)
	core.TestErr(t, err, "cannot reopen Bob's vault: %v", err)

	replicaAlice, err = Open(vAlice, dataDb)
	core.TestErr(t, err, "cannot reopen Alice's db: %v")

	replicaBob, err = Open(vBob, dataDb2)
	core.TestErr(t, err, "cannot reopen Bob's db: %v")

	// Alice inserts a new record
	_, err = replicaAlice.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "alice data 3", "cnt": 5, "ratio": 0.5, "bin": []byte{13, 14, 15}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err = replicaAlice.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	// Bob syncs and sees Alice's new data
	updates, err = replicaBob.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	rows, err = replicaBob.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 5, "expected 5 rows in Bob's replica, got %d", len(rows))
	core.Assert(t, rows[4][0] == "alice data 3", "unexpected value in last row: %s", rows[4][0])

	// Clean up
	vAlice.Close()
	vBob.Close()
	db.Close()
	db2.Close()
	dataDb.Close()
	dataDb2.Close()
	s.Close()
	s2.Close()
}

func TestExecWithSyncRelay(t *testing.T) {
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()

	db := sqlx.NewTestDB(t, "bao.db", "")
	dataDb := sqlx.NewTestDB(t, "vault2.db", testDdl)
	s, err := store.Open(store.LoadTestConfig(t, "test"))
	core.TestErr(t, err, "cannot open store: %v", err)
	defer s.Close()

	// Create vault for Alice
	v, err := vault.Create(vault.Users, aliceSecret, s, db, vault.Config{
		SyncRelay: "wss://sync-relay.baolib.org",
	})
	core.TestErr(t, err, "Create failed: %v")

	// Grant Bob read-write access
	err = v.SyncAccess(0,
		vault.AccessChange{Access: vault.ReadWrite, UserId: bob},
	)
	core.TestErr(t, err, "cannot set access: %v")

	// Alice opens replica and enables sync relay
	replica, err := Open(v, dataDb)
	core.TestErr(t, err, "cannot open db: %v")

	// Alice inserts data
	_, err = replica.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "relay test 1", "cnt": 1, "ratio": 0.5, "bin": []byte{1, 2, 3}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err := replica.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	// Bob opens vault and replica
	db2 := sqlx.NewTestDB(t, "vault3.db", "")
	dataDb2 := sqlx.NewTestDB(t, "vault4.db", testDdl)
	s2 := store.LoadTestStore(t, "test")

	vBob, err := vault.Open(vault.Users, bobSecret, alice, s2, db2)
	core.TestErr(t, err, "cannot open vault: %v")

	replicaBob, err := Open(vBob, dataDb2)
	core.TestErr(t, err, "cannot open db: %v")

	// Bob syncs and sees Alice's data
	updates, err = replicaBob.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	rows, err := replicaBob.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 1, "expected 1 row, got %d", len(rows))
	core.Assert(t, rows[0][0] == "relay test 1", "unexpected value: %s", rows[0][0])

	// Bob inserts data
	_, err = replicaBob.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "relay test 2", "cnt": 2, "ratio": 0.5, "bin": []byte{4, 5, 6}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err = replicaBob.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	// Alice syncs again with sync relay enabled
	updates, err = replica.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	rows, err = replica.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 2, "expected 2 rows, got %d", len(rows))
	core.Assert(t, rows[0][0] == "relay test 1", "unexpected value in first row: %s", rows[0][0])
	core.Assert(t, rows[1][0] == "relay test 2", "unexpected value in second row: %s", rows[1][0])

	// Both insert more data with relay enabled
	_, err = replica.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "relay test 3", "cnt": 3, "ratio": 0.5, "bin": []byte{7, 8, 9}})
	core.TestErr(t, err, "cannot insert test data: %v")

	_, err = replicaBob.Exec("INSERT_TEST_DATA", sqlx.Args{"msg": "relay test 4", "cnt": 4, "ratio": 0.5, "bin": []byte{10, 11, 12}})
	core.TestErr(t, err, "cannot insert test data: %v")

	updates, err = replica.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	updates, err = replicaBob.Sync()
	core.TestErr(t, err, "cannot sync: %v")
	core.Assert(t, updates == 1, "expected 1 update, got %d", updates)

	// Verify both have all 4 records with sync relay
	rows, err = replica.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 4, "expected 4 rows in Alice's replica with relay, got %d", len(rows))

	rows, err = replicaBob.Fetch("SELECT_TEST_DATA", sqlx.Args{}, 1000)
	core.TestErr(t, err, "cannot select test data: %v")
	core.Assert(t, len(rows) == 4, "expected 4 rows in Bob's replica with relay, got %d", len(rows))

	// Clean up
	v.Close()
	vBob.Close()
	db.Close()
	db2.Close()
	dataDb.Close()
	dataDb2.Close()
	s.Close()
	s2.Close()
}
