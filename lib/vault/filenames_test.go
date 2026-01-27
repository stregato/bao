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
		IsDir:     false,
		Flags:     PendingWrite,
		Attrs:     attrs,
		LocalCopy: "test.txt",
		StoreDir:  "store.dir",
		StoreName: "store.test.txt",
		AuthorId:  alice.PublicIDMust(),
		KeyId:     1, // Use a non-zero key ID for encryption
	}
	head, err := encodeHead(Users, file, alice, func(keyId uint64) (security.AESKey, error) {
		return aesKey, nil
	})
	core.TestErr(t, err, "cannot encode head: %v")

	keys := make(map[uint64]security.AESKey)
	keys[1] = aesKey
	file, err = decodeHead(Users, head, alice, func(keyId uint64) (security.AESKey, error) {
		return aesKey, nil
	}, func(shortId uint64) (security.PublicID, error) {
		return alice.PublicIDMust(), nil
	})
	core.TestErr(t, err, "cannot decode head: %v")
	core.Assert(t, file.Name == "test.txt", "unexpected name: %s", file.Name)
	core.Assert(t, file.Size == 1024, "unexpected size: %d", file.Size)
	core.Assert(t, file.AllocatedSize == 1024, "unexpected allocated size: %d", file.AllocatedSize)
	core.Assert(t, file.AuthorId == alice.PublicIDMust(), "unexpected author id: %s", file.AuthorId)
	core.Assert(t, file.ModTime.Sub(now) < time.Second, "unexpected mod time: %s", file.ModTime)
	core.Assert(t, string(file.Attrs) == string(attrs), "unexpected attrs: %s", file.Attrs)
}
