package mailbox

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/stregato/bao/lib/vault"
)

func TestMailbox(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)
	alice := security.NewPrivateIDMust()
	storeConfig := store.LoadTestConfig(t, "test")

	db := sqlx.NewTestDB(t, "mailbox.db", "")
	defer db.Close()
	store, err := store.Open(storeConfig)
	core.TestErr(t, err, "cannot open store: %v", err)
	defer store.Close()

	s, err := vault.Create(vault.Users, alice, store, db, vault.Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = s.SyncAccess(0, vault.AccessChange{Access: vault.ReadWrite, UserId: alice.PublicIDMust()})
	core.TestErr(t, err, "cannot set access: %v")

	err = Send(s, "testDir", Message{
		Subject: "test",
		Body:    "hello world",
	})
	core.TestErr(t, err, "cannot send message: %v")

	messages, err := Receive(s, "testDir", time.Time{}, 0)
	core.TestErr(t, err, "cannot receive messages: %v")
	core.Assert(t, len(messages) == 1, "unexpected number of messages")
	core.Assert(t, messages[0].Subject == "test", "unexpected subject")
	core.Assert(t, messages[0].Body == "hello world", "unexpected body")
}

func TestMailboxPair(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()
	carl, carlSecret := security.NewKeyPairMust()
	storeConfig := store.LoadTestConfig(t, "test")

	db := sqlx.NewTestDB(t, "mailbox.db", "")
	defer db.Close()
	store, err := store.Open(storeConfig)
	core.TestErr(t, err, "cannot open store: %v", err)
	defer store.Close()

	v, err := vault.Create(vault.Home, aliceSecret, store, db, vault.Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = v.SyncAccess(0, vault.AccessChange{Access: vault.ReadWrite, UserId: bob},
		vault.AccessChange{Access: vault.ReadWrite, UserId: carl})
	core.TestErr(t, err, "cannot set access: %v")

	err = Send(v, bob.String(), Message{
		Subject: "test",
		Body:    "hello world",
	})
	core.TestErr(t, err, "cannot send message: %v")

	v.Close()

	db = sqlx.NewTestDB(t, "mailbox2.db", "")
	defer db.Close()
	v, err = vault.Open(vault.Home, bobSecret, alice, store, db)
	core.TestErr(t, err, "Open failed: %v", err)

	messages, err := Receive(v, bob.String(), time.Time{}, 0)
	core.TestErr(t, err, "cannot receive messages: %v")
	core.Assert(t, len(messages) == 1, "unexpected number of messages")
	core.Assert(t, messages[0].Subject == "test", "unexpected subject")
	core.Assert(t, messages[0].Body == "hello world", "unexpected body")
	v.Close()

	db = sqlx.NewTestDB(t, "mailbox3.db", "")
	defer db.Close()
	v, err = vault.Open(vault.Home, carlSecret, alice, store, db)
	core.TestErr(t, err, "Open failed: %v", err)

	messages, err = Receive(v, bob.String(), time.Time{}, 0)
	core.Assert(t, err != nil, "access should be denied")
	v.Close()
}
