package vault

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func TestVersions(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed")

	// Create a temporary file with initial content
	tmpFile := t.TempDir() + "/versioned.txt"
	err = os.WriteFile(tmpFile, []byte("Version 1"), 0644)
	core.TestErr(t, err, "WriteFile failed")

	// Write the first version
	file1, err := v.Write("docs/versioned.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write version 1 failed")
	err = v.WaitFiles(file1.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Verify the file exists
	_, err = v.Stat("docs/versioned.txt")
	core.TestErr(t, err, "Stat after first write failed")

	// Sleep briefly to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Write the second version
	err = os.WriteFile(tmpFile, []byte("Version 2 - updated"), 0644)
	core.TestErr(t, err, "WriteFile for version 2 failed")
	file2, err := v.Write("docs/versioned.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write version 2 failed")
	err = v.WaitFiles(file2.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Sleep briefly to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Write the third version
	err = os.WriteFile(tmpFile, []byte("Version 3 - final"), 0644)
	core.TestErr(t, err, "WriteFile for version 3 failed")
	file3, err := v.Write("docs/versioned.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write version 3 failed")
	err = v.WaitFiles(file3.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Sync to populate the files table from storage
	_, err = v.Sync()
	core.TestErr(t, err, "Sync failed")

	// Get all versions
	versions, err := v.Versions("docs/versioned.txt")
	core.TestErr(t, err, "Versions failed")

	// Debug: print versions
	for i, ver := range versions {
		t.Logf("Version %d: ID=%d, Name=%s, Size=%d", i, ver.Id, ver.Name, ver.Size)
	}

	// Verify we have 3 versions
	core.Assert(t, len(versions) == 3, "expected 3 versions, got %d", len(versions))

	// Verify versions are ordered (oldest first by modTime - ASC order)
	core.Assert(t, versions[0].Id > 0, "version 1 should have valid ID")
	core.Assert(t, versions[1].Id > 0, "version 2 should have valid ID")
	core.Assert(t, versions[2].Id > 0, "version 3 should have valid ID")

	// Verify all versions have names in path:offset format
	// Both Versions() and GET_FILE_BY_NAME use ASC ordering (oldest→newest)
	// oldest (index 0) = offset 0, middle (index 1) = offset 1, newest (index 2) = offset 2
	core.Assert(t, versions[0].Name == "docs/versioned.txt:0", "version 1 name should be docs/versioned.txt:0, got %s", versions[0].Name)
	core.Assert(t, versions[1].Name == "docs/versioned.txt:1", "version 2 name should be docs/versioned.txt:1, got %s", versions[1].Name)
	core.Assert(t, versions[2].Name == "docs/versioned.txt:2", "version 3 name should be docs/versioned.txt:2, got %s", versions[2].Name)

	// Verify sizes match the content we wrote (oldest to newest)
	core.Assert(t, versions[0].Size == 9, "oldest version size should be 9, got %d", versions[0].Size)
	core.Assert(t, versions[1].Size == 19, "middle version size should be 19, got %d", versions[1].Size)
	core.Assert(t, versions[2].Size == 17, "latest version size should be 17, got %d", versions[2].Size)

	// Verify all versions have valid modification times
	for i, ver := range versions {
		core.Assert(t, !ver.ModTime.IsZero(), "version %d should have valid ModTime", i+1)
		core.Assert(t, ver.IsDir == false, "version %d should not be a directory", i+1)
	}

	// Test that the returned names can be used directly with Read
	tmpReadFile := t.TempDir() + "/read_version.txt"
	t.Logf("Reading version: %s (offset 0=oldest via ASC query)", versions[0].Name)
	_, err = v.Read(versions[0].Name, tmpReadFile, 0, nil) // Read oldest version (index 0, offset 0)
	core.TestErr(t, err, "WaitFiles after read failed")

	content, err := os.ReadFile(tmpReadFile)
	core.TestErr(t, err, "ReadFile failed")
	t.Logf("Read content: '%s' (expected 'Version 1')", string(content))
	core.Assert(t, string(content) == "Version 1", "oldest version content should be 'Version 1', got '%s'", string(content))

	v.Close()
	db.Close()
}

func TestVersionsNonExistentFile(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed")

	// Try to get versions of a file that doesn't exist
	versions, err := v.Versions("nonexistent/file.txt")
	core.TestErr(t, err, "Versions should not error on non-existent file")

	// Should return empty list
	core.Assert(t, len(versions) == 0, "expected 0 versions for non-existent file, got %d", len(versions))

	v.Close()
	db.Close()
}

func TestVersionsAfterDelete(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed")

	// Create a file with initial content
	tmpFile := t.TempDir() + "/deleted.txt"
	err = os.WriteFile(tmpFile, []byte("Original content"), 0644)
	core.TestErr(t, err, "WriteFile failed")

	// Write the file
	file1, err := v.Write("docs/deleted.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write failed")
	err = v.WaitFiles(file1.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Sleep briefly
	time.Sleep(100 * time.Millisecond)

	// Update the file
	err = os.WriteFile(tmpFile, []byte("Updated content"), 0644)
	core.TestErr(t, err, "WriteFile for update failed")
	file2, err := v.Write("docs/deleted.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write update failed")
	err = v.WaitFiles(file2.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Delete the file
	err = v.Delete("docs/deleted.txt", 0)
	core.TestErr(t, err, "Delete failed")

	// Sync to update the files table
	_, err = v.Sync()
	core.TestErr(t, err, "Sync failed")

	// Get versions - deleted versions should be filtered out
	versions, err := v.Versions("docs/deleted.txt")
	core.TestErr(t, err, "Versions failed")

	// Deleted files are filtered out in the Versions method
	// So we should have only non-deleted versions
	// Offsets match index: oldest at index 0 gets offset 0
	for i, ver := range versions {
		expectedName := "docs/deleted.txt:" + strconv.FormatUint(uint64(i), 10)
		core.Assert(t, ver.Name == expectedName, "version name should be %s, got %s", expectedName, ver.Name)
	}

	v.Close()
	db.Close()
}

func TestVersionsWithDifferentPaths(t *testing.T) {
	alice := security.NewPrivateIDMust()
	db := sqlx.NewTestDB(t, "vault.db", "")
	store := store.LoadTestStore(t, "test")
	defer store.Close()

	v, err := Create(Users, alice, store, db, Config{})
	core.TestErr(t, err, "Create failed")

	// Create files in different directories
	tmpFile := t.TempDir() + "/test.txt"
	err = os.WriteFile(tmpFile, []byte("Content 1"), 0644)
	core.TestErr(t, err, "WriteFile failed")

	// Write to root
	file1, err := v.Write("test.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write to root failed")
	err = v.WaitFiles(file1.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Write to subdirectory
	file2, err := v.Write("docs/test.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write to docs/ failed")
	err = v.WaitFiles(file2.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Write another version to subdirectory
	err = os.WriteFile(tmpFile, []byte("Content 2 updated"), 0644)
	core.TestErr(t, err, "WriteFile for update failed")
	file3, err := v.Write("docs/test.txt", tmpFile, nil, 0, nil)
	core.TestErr(t, err, "Write update to docs/ failed")
	err = v.WaitFiles(file3.Id)
	core.TestErr(t, err, "WaitFiles failed")

	// Sync to populate the files table
	_, err = v.Sync()
	core.TestErr(t, err, "Sync failed")

	// Get versions for root file
	versionsRoot, err := v.Versions("test.txt")
	core.TestErr(t, err, "Versions for root failed")
	core.Assert(t, len(versionsRoot) == 1, "expected 1 version in root, got %d", len(versionsRoot))

	// Get versions for subdirectory file
	versionsSubdir, err := v.Versions("docs/test.txt")
	core.TestErr(t, err, "Versions for docs/test.txt failed")
	core.Assert(t, len(versionsSubdir) == 2, "expected 2 versions in docs/, got %d", len(versionsSubdir))

	v.Close()
	db.Close()
}
