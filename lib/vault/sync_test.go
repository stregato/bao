package vault

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func TestStashSynchronize(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	// Setup a test DB (in-memory)
	db1 := sqlx.NewTestDB(t, "vault1.db", "")
	defer db1.Close()

	// Create a test vault
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	s, err := Create(Users, aliceSecret, store, db1, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = s.SetAttribute(0, "name", "alice")
	core.TestErr(t, err, "SetAttribute failed: %v")

	err = s.SyncAccess(0, AccessChange{Access: ReadWrite, UserId: bob})
	core.TestErr(t, err, "SyncAccess failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	os.WriteFile(tmpFile, []byte("Hello World"), 0644)

	// Write a file to the vault
	file, err := s.Write("simple.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write failed: %v")
	s.Close()

	// _, err = s.SyncGroups(Users) // Ensure the vault is synchronized before opening
	// core.TestErr(t, err, "SyncGroups failed: %v")
	err = s.WaitFiles(file.Id)
	core.TestErr(t, err, "WaitFiles failed: %v")

	db2 := sqlx.NewTestDB(t, "vault2.db", "")
	defer db2.Close()

	s, err = Open(Users, bobSecret, alice, store, db2)
	core.TestErr(t, err, "Open failed: %v")

	access, err := s.GetAccess(bob)
	core.TestErr(t, err, "GetAccess failed: %v")
	core.Assert(t, access == ReadWrite, "Expected ReadWrite access for Bob")

	// Call synchronize (should not error)
	newFiles, err := s.Sync()
	if err != nil {
		t.Errorf("synchronize failed: %v", err)
	}
	core.TestErr(t, err, "Synchronize should not error")
	core.Assert(t, len(newFiles) > 0, "Expected new files after synchronization")
	core.Assert(t, newFiles[0].Name == "simple.txt", "Expected file 'simple.txt' to be synchronized")

	newFiles, err = s.Sync()
	core.TestErr(t, err, "Synchronize should not error on second call")
	core.Assert(t, len(newFiles) == 0, "Expected no new files after second synchronization")
	s.Close()
}
