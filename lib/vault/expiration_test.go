package vault

import (
	"context"
	"testing"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func getExpirationUnixSec(t *testing.T, v *Vault, storeDir, storeName string) int64 {
	t.Helper()
	var exp int64
	err := v.DB.QueryRow("SQL:SELECT expiresAt FROM file_expirations WHERE vault=:vault AND storeDir=:storeDir AND storeName=:storeName",
		sqlx.Args{"vault": v.ID, "storeDir": storeDir, "storeName": storeName}, &exp)
	core.TestErr(t, err, "cannot read exact expiration for %s/%s: %v", storeDir, storeName, err)
	return exp
}

func getFileFlagsByStoreObject(t *testing.T, v *Vault, storeDir, storeName string) Flags {
	t.Helper()
	var flags Flags
	err := v.DB.QueryRow("SQL:SELECT flags FROM files WHERE vault=:vault AND storeDir=:storeDir AND storeName=:storeName ORDER BY id DESC LIMIT 1",
		sqlx.Args{"vault": v.ID, "storeDir": storeDir, "storeName": storeName}, &flags)
	core.TestErr(t, err, "cannot read flags for %s/%s: %v", storeDir, storeName, err)
	return flags
}

func TestFileExpirationCleanup(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault_expiration.db", "")
	st := store.LoadTestStore(t, "test")
	defer st.Close()

	v, err := Create(alice, st, db, Config{Retention: 2 * time.Hour})
	core.TestErr(t, err, "Create failed: %v")

	fDefault, err := v.Write("default-exp.txt", "", nil, IOOption{})
	core.TestErr(t, err, "Write default failed: %v")
	_, err = v.WaitFiles(context.Background(), fDefault.Id)
	core.TestErr(t, err, "WaitFiles default failed: %v")

	fShort, err := v.Write("short-exp.txt", "", nil, IOOption{Retention: 30 * time.Second})
	core.TestErr(t, err, "Write short failed: %v")
	_, err = v.WaitFiles(context.Background(), fShort.Id)
	core.TestErr(t, err, "WaitFiles short failed: %v")

	sDefault, err := v.Stat("default-exp.txt")
	core.TestErr(t, err, "Stat default failed: %v")
	sShort, err := v.Stat("short-exp.txt")
	core.TestErr(t, err, "Stat short failed: %v")

	expDefault := getExpirationUnixSec(t, v, sDefault.StoreDir, sDefault.StoreName)
	expShort := getExpirationUnixSec(t, v, sShort.StoreDir, sShort.StoreName)
	core.Assert(t, expShort < expDefault, "expected shorter retention to expire first: short=%d default=%d", expShort, expDefault)

	// Force only short file to be expired, then cleanup.
	err = v.setFileExpiration(sShort.StoreDir, sShort.StoreName, core.Now().Add(-5*time.Second))
	core.TestErr(t, err, "cannot force short expiration: %v", err)
	v.retentionCleanup()

	entries, err := v.ReadDir("", time.Time{}, 0, 100)
	core.TestErr(t, err, "ReadDir after first cleanup failed: %v", err)
	hasDefault := false
	hasShort := false
	for _, e := range entries {
		if e.Name == "default-exp.txt" {
			hasDefault = true
		}
		if e.Name == "short-exp.txt" {
			hasShort = true
		}
	}
	core.Assert(t, hasDefault, "expected default-exp.txt to remain after first cleanup")
	core.Assert(t, !hasShort, "expected short-exp.txt to be removed after first cleanup")
	core.Assert(t, getFileFlagsByStoreObject(t, v, sShort.StoreDir, sShort.StoreName)&Deleted != 0,
		"expected short-exp.txt row to be kept and marked deleted")

	// Expire the default file too.
	err = v.setFileExpiration(sDefault.StoreDir, sDefault.StoreName, core.Now().Add(-5*time.Second))
	core.TestErr(t, err, "cannot force default expiration: %v", err)
	v.retentionCleanup()

	entries, err = v.ReadDir("", time.Time{}, 0, 100)
	core.TestErr(t, err, "ReadDir after second cleanup failed: %v", err)
	core.Assert(t, len(entries) == 0, "expected all files to be removed after second cleanup, got %d", len(entries))
	core.Assert(t, getFileFlagsByStoreObject(t, v, sDefault.StoreDir, sDefault.StoreName)&Deleted != 0,
		"expected default-exp.txt row to be kept and marked deleted")
}
