package vault

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
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

	v, err := Create(aliceSecret, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = v.SyncAccess(IOOption{}, AccessChange{
		UserId: bob,
		Access: Read,
	})
	core.TestErr(t, err, "SyncAccess failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	file, err := v.Write("folder/simple.txt", tmpFile, attrs, IOOption{Scheduled: true})
	core.TestErr(t, err, "Write failed: %v")

	_, err = v.WaitFiles(context.Background(), file.Id)
	core.TestErr(t, err, "Sync failed: %v")
	v.Close()
	db.Close()

	db = sqlx.NewTestDB(t, "vault2.db", "")
	v, err = Open(bobSecret, alice, store, db)
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
	file, err = v.Read("folder/simple.txt", tmpFile2, IOOption{}, nil)
	core.TestErr(t, err, "Read failed: %v")
	core.Assert(t, file.Name == "folder/simple.txt", "")
	core.Assert(t, file.Size == 11, "")
	core.Assert(t, file.AllocatedSize >= file.Size, "")
	core.Assert(t, file.ModTime.Unix() > 0, "")
	core.Assert(t, file.IsDir == false, "")
	core.Assert(t, file.Id > 0, "")
	_, err = v.WaitFiles(context.Background(), file.Id)
	core.TestErr(t, err, "WaitFiles failed: %v")

	content, err := os.ReadFile(tmpFile2)
	core.TestErr(t, err, "ReadFile failed: %v")
	core.Assert(t, string(content) == "Hello World", "")

	v.Delete("simple.txt", IOOption{})
	v.Close()
	db.Close()
}

func TestWritePublic(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	s, err := Create(alice, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	file, err := s.Write("simple.txt", tmpFile, attrs, IOOption{})
	core.TestErr(t, err, "Write failed")

	_, err = s.WaitFiles(context.Background(), file.Id)
	core.TestErr(t, err, "Sync failed")
	s.Close()
	db.Close()
}

func TestWriteHome(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()

	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(aliceSecret, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	err = v.SyncAccess(IOOption{}, AccessChange{Access: ReadWrite, UserId: bob})
	core.TestErr(t, err, "SyncAccess failed: %v")

	tmpFile := t.TempDir() + "/simple.txt"
	err = os.WriteFile(tmpFile, []byte("Hello World"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")
	attrs := []byte{1, 2, 3, 4, 5}
	name := "shared/simple.txt,ec=" + bob.String()
	file, err := v.Write(name, tmpFile, attrs, IOOption{})
	core.TestErr(t, err, "Write failed")
	core.Assert(t, file.Flags&EcEncryption != 0, "Expected EC encryption flag")
	core.Assert(t, file.Flags&AESEncryption == 0, "Expected AES encryption flag to be disabled")
	core.Assert(t, file.EcRecipient == bob, "Expected EC recipient to be Bob")

	_, err = v.WaitFiles(context.Background(), file.Id)
	core.TestErr(t, err, "Sync failed")

	st, err := v.Stat("shared/simple.txt")
	core.TestErr(t, err, "Stat failed: %v")
	core.Assert(t, st.Flags&EcEncryption != 0, "Expected EC encryption flag in stat")
	core.Assert(t, st.EcRecipient == bob, "Expected EC recipient in stat")
	v.Close()
	db.Close()

	db = sqlx.NewTestDB(t, "vault2.db", "")
	v, err = Open(bobSecret, alice, store, db)
	core.TestErr(t, err, "Open failed: %v")

	ls, err := v.ReadDir("shared", time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(ls) == 1, "Expected one file in shared directory")
	core.Assert(t, ls[0].Name == "simple.txt", "Expected clean file name without encryption token")
	core.Assert(t, ls[0].Flags&EcEncryption != 0, "Expected EC encryption flag in directory entry")
	core.Assert(t, ls[0].EcRecipient == bob, "Expected EC recipient in directory entry")

	tmpFile = t.TempDir() + "/simple2.txt"
	_, err = v.Read(name, tmpFile, IOOption{}, nil)
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

	s, err := Create(alice, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	attrs := []byte{1, 2, 3, 4, 5}
	file, err := s.Write("attrs.txt", "", attrs, IOOption{})
	core.TestErr(t, err, "Write failed: %v")
	core.Assert(t, file.Name == "attrs.txt", "Expected file name to be 'attrs.txt'")
	core.Assert(t, bytes.Equal(file.Attrs, attrs), "Expected file attrs data to match")

	files, err := s.ReadDir("", time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(files) == 1, "Expected one file in directory")
	core.Assert(t, files[0].Name == "attrs.txt", "Expected file name to be 'attrs.txt'")
	core.Assert(t, bytes.Equal(files[0].Attrs, attrs), "Expected file attrs data to match")

	err = s.Delete("attrs.txt", IOOption{})
	core.TestErr(t, err, "Delete failed: %v")

	// Verify the file is deleted
	files, err = s.ReadDir("", time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(files) == 0, "Expected no files after delete")
	s.Close()
}

func TestWriteRetentionOverride(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault_retention.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(alice, store, db, Config{Retention: 24 * time.Hour})
	core.TestErr(t, err, "Create failed: %v")

	now := core.Now()
	file, err := v.Write("retention.txt", "", nil, IOOption{Retention: 2 * time.Hour})
	core.TestErr(t, err, "Write failed: %v")

	expected := truncateToSecond(now.Add(2 * time.Hour))
	diff := file.ExpiresAt.Sub(expected)
	if diff < 0 {
		diff = -diff
	}
	core.Assert(t, diff <= time.Second, "expected expiry around %s, got %s", expected, file.ExpiresAt)
	v.Close()
}

func TestWriteNestedPath(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault_nested.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(alice, store, db, Config{})
	core.TestErr(t, err, "Create failed: %v")

	tmpFile := t.TempDir() + "/nested.txt"
	err = os.WriteFile(tmpFile, []byte("Nested content"), 0644)
	core.TestErr(t, err, "WriteFile failed: %v")

	file, err := v.Write("level1/level2/nested.txt", tmpFile, nil, IOOption{})
	core.TestErr(t, err, "Write failed: %v")

	_, err = v.WaitFiles(context.Background(), file.Id)
	core.TestErr(t, err, "WaitFiles failed: %v")

	f, err := v.Stat("level1/level2/nested.txt")
	core.TestErr(t, err, "Stat failed: %v")
	core.Assert(t, f.Name == "level1/level2/nested.txt", "Expected nested file name")
	core.Assert(t, f.IsDir == false, "Expected regular file")

	entries, err := v.ReadDir("level1/level2", time.Time{}, 0, 0)
	core.TestErr(t, err, "ReadDir failed: %v")
	core.Assert(t, len(entries) == 1, "Expected one file in nested directory")
	core.Assert(t, entries[0].Name == "nested.txt", "Expected nested file entry")

	v.Close()
	db.Close()
}

func TestWriteWithSyncRelay(t *testing.T) {
	requireSyncRelay(t, "wss://sync-relay.baolib.org")

	alice, aliceSecret, err := security.NewKeyPair()
	core.TestErr(t, err, "cannot create keys")
	bob, bobSecret, err := security.NewKeyPair()
	core.TestErr(t, err, "cannot create keys")

	// Setup: Alice creates a vault with sync relay
	db1 := sqlx.NewTestDB(t, "vault_alice.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	config := Config{
		// SyncRelay: "ws://localhost:8787",
		SyncRelay: "wss://sync-relay.baolib.org",
	}

	va, err := Create(aliceSecret, store, db1, config)
	core.TestErr(t, err, "Alice: Create failed: %v")

	// Setup: Bob opens the same vault with sync relay
	db2 := sqlx.NewTestDB(t, "vault_bob.db", "")
	vb, err := Open(bobSecret, alice, store, db2)
	core.TestErr(t, err, "Bob: Open failed: %v")

	// Give both sync relays a moment to connect
	time.Sleep(time.Second)

	// Alice grants Bob access
	err = va.SyncAccess(IOOption{}, AccessChange{
		UserId: bob,
		Access: Read,
	})
	core.TestErr(t, err, "Alice: SyncAccess failed: %v")

	// Alice writes a file
	tmpFile := t.TempDir() + "/relay-test.txt"
	err = os.WriteFile(tmpFile, []byte("Testing sync relay"), 0644)
	core.TestErr(t, err, "Alice: WriteFile failed: %v")

	_, err = va.Write("relay/test.txt", tmpFile, nil, IOOption{})
	core.TestErr(t, err, "Alice: Write failed: %v")

	time.Sleep(time.Second)

	// Bob reads directory and should see Alice's file
	files, err := vb.ReadDir("relay", time.Time{}, 0, 0)
	core.TestErr(t, err, "Bob: ReadDir failed: %v")
	core.Assert(t, len(files) == 1, "Bob: Expected one file in directory, got %d", len(files))
	core.Assert(t, files[0].Name == "test.txt", "Bob: Expected file name to be 'test.txt'")
	core.Assert(t, files[0].Size == 18, "Bob: Expected file size to be 18 bytes")

	// Bob reads the file
	readFile := t.TempDir() + "/relay-test-read.txt"
	_, err = vb.Read("relay/test.txt", readFile, IOOption{}, nil)
	core.TestErr(t, err, "Bob: Read failed: %v")

	content, err := os.ReadFile(readFile)
	core.TestErr(t, err, "Bob: ReadFile failed: %v")
	core.Assert(t, string(content) == "Testing sync relay", "Bob: Expected file content to match")

	// Clean up
	va.Close()
	vb.Close()
	db1.Close()
	db2.Close()
}
