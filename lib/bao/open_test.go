package bao

import (
	"testing"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

func TestOpen(t *testing.T) {
	db := sqlx.NewTestDB(t, "stash.db", "")
	alice := security.NewPrivateIDMust()

	tmpFolder := t.TempDir()
	storeUrl := "file://" + tmpFolder
	s, err := Create(db, alice, storeUrl, Config{})
	core.TestErr(t, err, "cannot create stash")

	err = s.Close()
	core.TestErr(t, err, "cannot close stash")

	bob := security.NewPrivateIDMust()

	s, err = Open(db, bob, storeUrl, alice.PublicIDMust())
	core.TestErr(t, err, "cannot open stash")

	accesses, err := s.GetUsers(Admins)
	core.TestErr(t, err, "cannot get accesses")
	core.Assert(t, len(accesses) == 1, "expected 1 access, got %d", len(accesses))

	err = s.Close()
	core.TestErr(t, err, "cannot close stash")
}
