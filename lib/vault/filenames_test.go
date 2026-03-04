package vault

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
)

func TestEncodeDecodeHead(t *testing.T) {
	alice := security.NewPrivateIDMust()
	attrs := []byte("attrs-data")
	aesKey := make(security.AESKey, 32)
	_, err := rand.Read(aesKey)
	core.TestErr(t, err, "cannot generate AES key: %v")

	now := time.Now()
	file := File{
		Name:      "test.txt",
		Size:      1024,
		ModTime:   now,
		ExpiresAt: truncateToSecond(now.Add(90 * time.Minute)),
		IsDir:     false,
		Flags:     PendingWrite,
		Attrs:     attrs,
		LocalCopy: "test.txt",
		StoreDir:  "store.dir",
		StoreName: "store.test.txt",
		AuthorId:  alice.PublicIDMust(),
		KeyId:     1, // Use a non-zero key ID for encryption
	}
	head, err := encodeHead("aes", file, "", alice, func(keyId uint64) (security.AESKey, error) {
		return aesKey, nil
	})
	core.TestErr(t, err, "cannot encode head: %v")

	keys := make(map[uint64]security.AESKey)
	keys[1] = aesKey
	var notForMe bool
	var retryAfterBlockchain bool
	file, notForMe, retryAfterBlockchain, err = decodeHead(head, alice, func(keyId uint64) (security.AESKey, error) {
		return aesKey, nil
	}, func(shortId uint64) (security.PublicID, error) {
		return alice.PublicIDMust(), nil
	})
	core.TestErr(t, err, "cannot decode head: %v")
	core.Assert(t, !notForMe, "expected file to be for current user")
	core.Assert(t, !retryAfterBlockchain, "did not expect blockchain retry condition")
	core.Assert(t, file.Name == "test.txt", "unexpected name: %s", file.Name)
	core.Assert(t, file.Size == 1024, "unexpected size: %d", file.Size)
	core.Assert(t, file.AllocatedSize == 1024, "unexpected allocated size: %d", file.AllocatedSize)
	core.Assert(t, file.AuthorId == alice.PublicIDMust(), "unexpected author id: %s", file.AuthorId)
	core.Assert(t, file.ModTime.Sub(now) < time.Second, "unexpected mod time: %s", file.ModTime)
	core.Assert(t, file.ExpiresAt.Equal(truncateToSecond(now.Add(90*time.Minute))), "unexpected expiresAt: %s", file.ExpiresAt)
	core.Assert(t, string(file.Attrs) == string(attrs), "unexpected attrs: %s", file.Attrs)

	// Expiration is stored in the clear header prefix, so it is available even when
	// this user cannot decrypt the payload.
	notForMeFile, notForMe, _, err := decodeHead(head, alice, func(keyId uint64) (security.AESKey, error) {
		return nil, core.Error(core.AccessDenied, "no access to key")
	}, func(shortId uint64) (security.PublicID, error) {
		return alice.PublicIDMust(), nil
	})
	core.TestErr(t, err, "cannot decode not-for-me head: %v")
	core.Assert(t, notForMe, "expected file to be not-for-me")
	core.Assert(t, notForMeFile.ExpiresAt.Equal(truncateToSecond(now.Add(90*time.Minute))), "unexpected not-for-me expiresAt: %s", notForMeFile.ExpiresAt)
}
