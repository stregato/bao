package bao

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

func TestStashWrite(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	storeConfig := storage.LoadTestConfig(t, "test")

	s, err := Create(db, alice, storeConfig, Config{})
	core.TestErr(t, err, "Create failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	file, err := s.Write("simple.txt", tmpFile, Admins, attrs, ScheduledOperation, nil)
	core.TestErr(t, err, "Write failed: %v")

	err = s.WaitFiles(file.Id)
	core.TestErr(t, err, "Sync failed: %v")
	// core.Assert(t, len(files) == 1, "Expected one file after sync")
	// core.Assert(t, files[0].Name == "simple.txt", "Expected file name to be 'simple.txt'")
	// core.Assert(t, files[0].Size == 11, "Expected file size to be 11")
	// core.Assert(t, files[0].ModTime.Unix() > 0, "Expected file mod time to be set")
	// core.Assert(t, files[0].IsDir == false, "Expected file to not be a directory")
	// core.Assert(t, files[0].Id > 0, "Expected file ID to be greater than 0")
	// core.Assert(t, len(files[0].Attrs) == len(attrs), "Expected file attrs data to match")
	// core.Assert(t, bytes.Equal(files[0].Attrs, attrs), "Expected file attrs data to match")

	ls, err := s.ReadDir("", time.Time{}, 0, 0)
	core.TestErr(t, err, "")
	core.Assert(t, len(ls) == 1, "")

	f, err := s.Stat("simple.txt")
	core.TestErr(t, err, "Stat failed: %v")
	core.Assert(t, f.Name == "simple.txt", "")
	core.Assert(t, f.Size == 11, "")
	core.Assert(t, f.AllocatedSize >= f.Size, "")
	core.Assert(t, f.ModTime.Unix() > 0, "")
	core.Assert(t, f.IsDir == false, "")
	core.Assert(t, f.Id > 0, "")

	tmpFile2 := t.TempDir() + "/simple2.txt"
	file, err = s.Read("simple.txt", tmpFile2, AsyncOperation, nil)
	core.TestErr(t, err, "Read failed: %v")
	core.Assert(t, file.Name == "simple.txt", "")
	core.Assert(t, file.Size == 11, "")
	core.Assert(t, file.AllocatedSize >= file.Size, "")
	core.Assert(t, file.ModTime.Unix() > 0, "")
	core.Assert(t, file.IsDir == false, "")
	core.Assert(t, file.Id > 0, "")
	s.WaitFiles(file.Id)

	content, err := os.ReadFile(tmpFile2)
	core.TestErr(t, err, "ReadFile failed: %v")
	core.Assert(t, string(content) == "Hello World", "")

	s.Delete("simple.txt", 0)

	s.Close()
}

func TestWritePublic(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	storeConfig := storage.LoadTestConfig(t, "test")

	s, err := Create(db, alice, storeConfig, Config{})
	core.TestErr(t, err, "Create failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	file, err := s.Write("simple.txt", tmpFile, Public, attrs, 0, nil)
	core.TestErr(t, err, "Write failed")

	err = s.WaitFiles(file.Id)
	core.TestErr(t, err, "Sync failed")
	s.Close()
	db.Close()
}

func TestWriteAttrs(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	storeConfig := storage.LoadTestConfig(t, "test")

	s, err := Create(db, alice, storeConfig, Config{})
	core.TestErr(t, err, "Create failed: %v")

	attrs := []byte{1, 2, 3, 4, 5}
	file, err := s.Write("attrs.txt", "", Admins, attrs, 0, nil)
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
	changes, err := s.Sync(Admins)
	core.TestErr(t, err, "Sync failed: %v")
	core.Assert(t, len(changes) == 1, "Expected one change after delete")

	// Verify the file is deleted
	files, err = s.ReadDir("", time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(files) == 0, "Expected no files after delete")
	s.Close()
}
