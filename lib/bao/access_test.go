package bao

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

func TestAccess(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	storeConfig := storage.StoreConfig{
		Id:   "local-test-store",
		Type: "local",
		Local: storage.LocalConfig{
			Base: "file://" + t.TempDir(),
		},
	}

	s, err := Create(db, alice, storeConfig, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = s.SyncAccess(0, AccessChange{Group: Users, Access: ReadWrite, UserId: alice.PublicIDMust()})
	core.TestErr(t, err, "SyncAccess failed: %v")

	accesses, err := s.GetUsers(Users)
	core.TestErr(t, err, "GetAccess failed: %v")

	core.Assert(t, len(accesses) == 1, "One user should have access")
	core.Assert(t, accesses[alice.PublicIDMust()] == Read+Write, "")

}

func TestAccessTwoUsers(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	alice := security.NewPrivateIDMust()
	alicePublic := alice.PublicIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")

	storeConfig := storage.StoreConfig{
		Id:   "local-test-store",
		Type: "local",
		Local: storage.LocalConfig{
			Base: "file://" + t.TempDir(),
		},
	}

	sa, err := Create(db, alice, storeConfig, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = sa.SyncAccess(0, AccessChange{Group: Users, Access: ReadWrite, UserId: alicePublic})
	core.TestErr(t, err, "SyncAccess failed: %v")
	sa.Close()

	bob := security.NewPrivateIDMust()
	bobPublic := bob.PublicIDMust()
	sb, err := Open(db, bob, storeConfig, alicePublic)
	core.TestErr(t, err, "Open failed: %v")

	groups, err := sb.GetGroups(bobPublic)
	core.TestErr(t, err, "GetAccess failed: %v")

	core.Assert(t, len(groups) == 0, "Bob should not have access to any group")
	sb.Close()

	err = sa.SyncAccess(0, AccessChange{Group: Users, Access: ReadWrite, UserId: bob.PublicIDMust()})
	core.TestErr(t, err, "SyncAccess failed: %v")

	sb, err = Open(db, bob, storeConfig, alice.PublicIDMust())
	core.TestErr(t, err, "Open failed: %v")

	groups, err = sb.GetGroups(bobPublic)
	core.TestErr(t, err, "GetAccess failed: %v")

	core.Assert(t, len(groups) == 1, "Bob should have access to one group")
	core.Assert(t, groups[Users] == Read+Write, "Bob should have read and write access")
	sb.Close()
}
