package bao

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

func TestStashSynchronize(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	// Setup a test DB (in-memory)
	db1 := sqlx.NewTestDB(t, "stash1.db", "")
	defer db1.Close()

	// Create a test stash
	alice, _ := security.NewPrivateID()
	bob, _ := security.NewPrivateID()
	storeUrl := "file://" + t.TempDir()

	s, err := Create(db1, alice, storeUrl, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = s.SetAttribute(0, "name", "alice")
	core.TestErr(t, err, "SetAttribute failed: %v")

	err = s.SyncAccess(0, AccessChange{Group: Users, Access: ReadWrite, UserId: alice.PublicIDMust()})
	core.TestErr(t, err, "SyncAccess failed: %v")

	err = s.SyncAccess(0, AccessChange{Group: Users, Access: ReadWrite, UserId: bob.PublicIDMust()})
	core.TestErr(t, err, "SyncAccess failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	os.WriteFile(tmpFile, []byte("Hello World"), 0644)

	// Write a file to the stash
	file, err := s.Write("simple.txt", tmpFile, Users, nil, 0, nil)
	core.TestErr(t, err, "Write failed: %v")
	s.Close()

	// _, err = s.SyncGroups(Users) // Ensure the stash is synchronized before opening
	// core.TestErr(t, err, "SyncGroups failed: %v")
	err = s.WaitFiles(file.Id)
	core.TestErr(t, err, "WaitFiles failed: %v")

	db2 := sqlx.NewTestDB(t, "stash2.db", "")
	defer db2.Close()

	s, err = Open(db2, bob, storeUrl, alice.PublicIDMust())
	core.TestErr(t, err, "Open failed: %v")

	groups, err := s.GetGroups(bob.PublicIDMust())
	core.TestErr(t, err, "GetGroups failed: %v")
	core.Assert(t, len(groups) == 1, "Expected 1 group for bob, got %d", len(groups))
	core.Assert(t, groups[Users] > 0, "Expected access to group 'Users' for bob, got %s", groups[Users])

	// Call synchronize (should not error)
	newFiles, err := s.Sync(Users)
	if err != nil {
		t.Errorf("synchronize failed: %v", err)
	}
	core.TestErr(t, err, "Synchronize should not error")
	core.Assert(t, len(newFiles) > 0, "Expected new files after synchronization")
	core.Assert(t, newFiles[0].Name == "simple.txt", "Expected file 'simple.txt' to be synchronized")

	newFiles, err = s.Sync(Users)
	core.TestErr(t, err, "Synchronize should not error on second call")
	core.Assert(t, len(newFiles) == 0, "Expected no new files after second synchronization")
	s.Close()
}
