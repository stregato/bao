package vault

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func TestCreate(t *testing.T) {

	db := sqlx.NewTestDB(t, "vault.db", "")
	alice := security.NewPrivateIDMust()

	store := store.LoadTestStore(t, "test")
	defer store.Close()

	s, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "cannot create vault")

	accesses, err := s.GetAccesses()
	core.TestErr(t, err, "cannot get accesses")
	core.Assert(t, len(accesses) == 1, "expected 1 access, got %d", len(accesses))

	err = s.Close()
	core.TestErr(t, err, "cannot close vault")
}
