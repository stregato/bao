package vault

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func TestOpen(t *testing.T) {
	db := sqlx.NewTestDB(t, "vault.db", "")
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()

	store := store.LoadTestStore(t, "test")
	defer store.Close()

	s, err := Create(Users, aliceSecret, store, db, Config{})
	core.TestErr(t, err, "cannot create vault")

	err = s.SyncAccess(0, AccessChange{Access: ReadWrite, UserId: bob})
	core.TestErr(t, err, "cannot set access")

	err = s.Close()
	core.TestErr(t, err, "cannot close vault")

	s, err = Open(Users, bobSecret, alice, store, db)
	core.TestErr(t, err, "cannot open vault")

	accesses, err := s.GetAccesses()
	core.TestErr(t, err, "cannot get accesses")
	core.Assert(t, len(accesses) == 2, "expected 2 accesses, got %d", len(accesses))

	err = s.Close()
	core.TestErr(t, err, "cannot close vault")
}
