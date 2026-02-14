package vault

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func TestWaitUpdates(t *testing.T) {
	// Setup test environment
	db := sqlx.NewTestDB(t, "vault.db", "")
	defer db.Close()

	alice, aliceSecret := security.NewKeyPairMust()
	bob, bobSecret := security.NewKeyPairMust()
	s, err := store.Open(store.LoadTestConfig(t, "test"))
	core.TestErr(t, err, "cannot open store: %v", err)
	defer s.Close()

	// Create vault for Alice with sync relay
	vAlice, err := Create(Users, aliceSecret, s, db, Config{
		SyncRelay: "wss://sync-relay.baolib.org",
	})
	core.TestErr(t, err, "Create failed: %v")
	defer vAlice.Close()

	// Grant Bob access
	err = vAlice.SyncAccess(0, AccessChange{Access: ReadWrite, UserId: bob})
	core.TestErr(t, err, "cannot set access: %v")

	// Bob opens the vault with sync relay
	db2 := sqlx.NewTestDB(t, "vault2.db", "")
	defer db2.Close()
	s2 := store.LoadTestStore(t, "test")
	defer s2.Close()

	vBob, err := Open(Users, bobSecret, alice, s2, db2)
	core.TestErr(t, err, "cannot open vault: %v")
	defer vBob.Close()

	// Test 1: WaitUpdates with timeout (no updates)
	t.Run("TimeoutNoUpdates", func(t *testing.T) {
		hasUpdates := vBob.WaitUpdates(100 * time.Millisecond)
		core.Assert(t, !hasUpdates, "expected timeout without updates")
	})

	// Test 2: WaitUpdates returns true when file arrives
	t.Run("UpdatesArrive", func(t *testing.T) {
		// Start waiting in a goroutine
		resultCh := make(chan bool, 1)
		go func() {
			hasUpdates := vBob.WaitUpdates(5 * time.Second)
			resultCh <- hasUpdates
		}()

		// Give the goroutine time to start waiting
		time.Sleep(100 * time.Millisecond)

		// Alice writes a file
		tmpFile := t.TempDir() + "/test.txt"
		os.WriteFile(tmpFile, []byte("test content"), 0644)
		file, err := vAlice.Write("test.txt", tmpFile, nil, 0, nil)
		core.TestErr(t, err, "Write failed: %v")

		// Wait for file to be written
		_, err = vAlice.WaitFiles(context.Background(), file.Id)
		core.TestErr(t, err, "WaitFiles failed: %v")

		// Bob should be notified
		select {
		case hasUpdates := <-resultCh:
			core.Assert(t, hasUpdates, "expected updates notification")
		case <-time.After(2 * time.Second):
			t.Fatal("WaitUpdates did not return after file sync")
		}
	})

	// Test 3: Multiple waiters get notified
	t.Run("MultipleWaiters", func(t *testing.T) {
		numWaiters := 3
		results := make(chan bool, numWaiters)

		// Start multiple waiters
		for i := 0; i < numWaiters; i++ {
			go func() {
				hasUpdates := vBob.WaitUpdates(5 * time.Second)
				results <- hasUpdates
			}()
		}

		// Give goroutines time to start waiting
		time.Sleep(100 * time.Millisecond)

		// Alice writes another file
		tmpFile := t.TempDir() + "/test2.txt"
		os.WriteFile(tmpFile, []byte("test content 2"), 0644)
		file, err := vAlice.Write("test2.txt", tmpFile, nil, 0, nil)
		core.TestErr(t, err, "Write failed: %v")

		_, err = vAlice.WaitFiles(context.Background(), file.Id)
		core.TestErr(t, err, "WaitFiles failed: %v")

		// All waiters should be notified
		notifiedCount := 0
		timeout := time.After(2 * time.Second)
		for i := 0; i < numWaiters; i++ {
			select {
			case hasUpdates := <-results:
				if hasUpdates {
					notifiedCount++
				}
			case <-timeout:
				t.Fatalf("only %d/%d waiters were notified", notifiedCount, numWaiters)
			}
		}
		core.Assert(t, notifiedCount == numWaiters, "expected all %d waiters to be notified, got %d", numWaiters, notifiedCount)
	})
}
