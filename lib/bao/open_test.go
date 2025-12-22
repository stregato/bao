package bao

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

func TestOpen(t *testing.T) {
	db := sqlx.NewTestDB(t, "vault.db", "")
	alice := security.NewPrivateIDMust()

	storeConfig := storage.LoadTestConfig(t, "test")
	s, err := Create(db, alice, storeConfig, Config{})
	core.TestErr(t, err, "cannot create vault")

	err = s.Close()
	core.TestErr(t, err, "cannot close vault")

	bob := security.NewPrivateIDMust()

	s, err = Open(db, bob, storeConfig, alice.PublicIDMust())
	core.TestErr(t, err, "cannot open vault")

	accesses, err := s.GetUsers(Admins)
	core.TestErr(t, err, "cannot get accesses")
	core.Assert(t, len(accesses) == 1, "expected 1 access, got %d", len(accesses))

	err = s.Close()
	core.TestErr(t, err, "cannot close vault")
}
