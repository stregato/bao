package vault

import (
	"bytes"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func TestVaultWrite(t *testing.T) {
	alice, aliceSecret, err := security.NewKeyPair()
	core.TestErr(t, err, "cannot create keys")
	bob, bobSecret, err := security.NewKeyPair()
	core.TestErr(t, err, "cannot create keys")

	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(Users, aliceSecret, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = v.SyncAccess(0, AccessChange{
		UserId: bob,
		Access: Read,
	})
	core.TestErr(t, err, "SyncAccess failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	file, err := v.Write("folder/simple.txt", tmpFile, attrs, ScheduledOperation, nil)
	core.TestErr(t, err, "Write failed: %v")

	err = v.WaitFiles(file.Id)
	core.TestErr(t, err, "Sync failed: %v")
	v.Close()
	db.Close()

	db = sqlx.NewTestDB(t, "vault2.db", "")
	v, err = Open(Users, bobSecret, alice, store, db)
	core.TestErr(t, err, "Open failed: %v")

	ls, err := v.ReadDir("folder", time.Time{}, 0, 0)
	core.TestErr(t, err, "")
	core.Assert(t, len(ls) == 1, "")

	f, err := v.Stat("folder/simple.txt")
	core.TestErr(t, err, "Stat failed: %v")
	core.Assert(t, f.Name == "folder/simple.txt", "")
	core.Assert(t, f.Size == 11, "")
	core.Assert(t, f.AllocatedSize >= f.Size, "")
	core.Assert(t, f.ModTime.Unix() > 0, "")
	core.Assert(t, f.IsDir == false, "")
	core.Assert(t, f.Id > 0, "")

	tmpFile2 := t.TempDir() + "/simple2.txt"
	file, err = v.Read("folder/simple.txt", tmpFile2, 0, nil)
	core.TestErr(t, err, "Read failed: %v")
	core.Assert(t, file.Name == "folder/simple.txt", "")
	core.Assert(t, file.Size == 11, "")
	core.Assert(t, file.AllocatedSize >= file.Size, "")
	core.Assert(t, file.ModTime.Unix() > 0, "")
	core.Assert(t, file.IsDir == false, "")
	core.Assert(t, file.Id > 0, "")
	err = v.WaitFiles(file.Id)
	core.TestErr(t, err, "WaitFiles failed: %v")

	content, err := os.ReadFile(tmpFile2)
	core.TestErr(t, err, "ReadFile failed: %v")
	core.Assert(t, string(content) == "Hello World", "")

	v.Delete("simple.txt", 0)
	v.Close()
	db.Close()
}

func TestWritePublic(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	s, err := Create(All, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	file, err := s.Write("simple.txt", tmpFile, attrs, 0, nil)
	core.TestErr(t, err, "Write failed")

	err = s.WaitFiles(file.Id)
	core.TestErr(t, err, "Sync failed")
	s.Close()
	db.Close()
}

func TestWriteHome(t *testing.T) {
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()

	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(Home, aliceSecret, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = v.SyncAccess(0, AccessChange{Access: ReadWrite, UserId: bob})
	core.TestErr(t, err, "SyncAccess failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	file, err := v.Write(path.Join(bob.String(), "simple.txt"), tmpFile, attrs, 0, nil)
	core.TestErr(t, err, "Write failed")

	err = v.WaitFiles(file.Id)
	core.TestErr(t, err, "Sync failed")
	v.Close()
	db.Close()

	db = sqlx.NewTestDB(t, "vault2.db", "")
	v, err = Open(Home, bobSecret, alice, store, db)
	core.TestErr(t, err, "Open failed: %v")

	ls, err := v.ReadDir(bob.String(), time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(ls) == 1, "Expected one file in Bob's home directory")
	core.Assert(t, ls[0].Name == "simple.txt", "Expected file name to be 'simple.txt'")

	tmpFile = t.TempDir() + "/simple2.txt"
	_, err = v.Read(path.Join(bob.String(), "simple.txt"), tmpFile, 0, nil)
	core.TestErr(t, err, "Read failed: %v")

	content, err := os.ReadFile(tmpFile)
	core.TestErr(t, err, "ReadFile failed: %v")
	core.Assert(t, string(content) == "Hello World", "Expected file content to be 'Hello World'")

	v.Close()
}

func TestWriteAttrs(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	s, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	attrs := []byte{1, 2, 3, 4, 5}
	file, err := s.Write("attrs.txt", "", attrs, 0, nil)
	core.TestErr(t, err, "Write failed: %v")
	core.Assert(t, file.Name == "attrs.txt", "Expected file name to be 'attrs.txt'")
	core.Assert(t, bytes.Equal(file.Attrs, attrs), "Expected file attrs data to match")

	files, err := s.ReadDir("", time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(files) == 1, "Expected one file in directory")
	core.Assert(t, files[0].Name == "attrs.txt", "Expected file name to be 'attrs.txt'")
	core.Assert(t, bytes.Equal(files[0].Attrs, attrs), "Expected file attrs data to match")

	err = s.Delete("attrs.txt", 0)
	core.TestErr(t, err, "Delete failed: %v")

	// Verify the file is deleted
	files, err = s.ReadDir("", time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(files) == 0, "Expected no files after delete")
	s.Close()
}
