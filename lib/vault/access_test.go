package vault

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func TestAccess(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store, err := store.Open(store.StoreConfig{
		Id:   "local-test-store",
		Type: "local",
		Local: store.LocalConfig{
			Base: "file://" + t.TempDir(),
		},
	})
	core.TestErr(t, err, "cannot open store: %v", err)
	defer store.Close()

	s, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = s.SyncAccess(0, AccessChange{Access: ReadWrite, UserId: alice.PublicIDMust()})
	core.TestErr(t, err, "SyncAccess failed: %v")

	accesses, err := s.GetAccesses()
	core.TestErr(t, err, "GetAccess failed: %v")

	core.Assert(t, len(accesses) == 1, "One user should have access")
	core.Assert(t, accesses[alice.PublicIDMust()] == Read+Write, "")

}

func TestAccessTwoUsers(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	alice := security.NewPrivateIDMust()
	alicePublic := alice.PublicIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")

	store, err := store.Open(store.StoreConfig{
		Id:   "local-test-store",
		Type: "local",
		Local: store.LocalConfig{
			Base: "file://" + t.TempDir(),
		},
	})
	core.TestErr(t, err, "cannot open store: %v", err)

	sa, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = sa.SyncAccess(0, AccessChange{Access: ReadWrite, UserId: alicePublic})
	core.TestErr(t, err, "SyncAccess failed: %v")
	sa.Close()

	bob := security.NewPrivateIDMust()
	bobPublic := bob.PublicIDMust()
	sb, err := Open(Users, bob, alicePublic, store, db)
	core.TestErr(t, err, "Open failed: %v")

	access, err := sb.GetAccess(bobPublic)
	core.TestErr(t, err, "GetAccess failed: %v")
	core.Assert(t, access == 0, "Bob should have no access")
	sb.Close()

	err = sa.SyncAccess(0, AccessChange{Access: ReadWrite, UserId: bob.PublicIDMust()})
	core.TestErr(t, err, "SyncAccess failed: %v")

	sb, err = Open(Users, bob, alicePublic, store, db)
	core.TestErr(t, err, "Open failed: %v")

	access, err = sb.GetAccess(bobPublic)
	core.TestErr(t, err, "GetAccess failed: %v")

	core.Assert(t, access == Read+Write, "Bob should have read and write access")
	sb.Close()
}
