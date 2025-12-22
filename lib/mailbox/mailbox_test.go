package mailbox

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/bao"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

func TestMailbox(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)
	alice := security.NewPrivateIDMust()
	storeConfig := storage.LoadTestConfig(t, "test")

	db := sqlx.NewTestDB(t, "mailbox.db", "")
	defer db.Close()
	s, err := bao.Create(db, alice, storeConfig, bao.Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = s.SyncAccess(0, bao.AccessChange{Group: bao.Users, Access: bao.ReadWrite, UserId: alice.PublicIDMust()})
	core.TestErr(t, err, "cannot set access: %v")

	err = Send(s, "testDir", bao.Users, Message{
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
