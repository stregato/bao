package vault

import (
	"bytes"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto/blake2b"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
)

func TestCreateAndParseBlock(t *testing.T) {
	id := security.NewPrivateIDMust()
	hash := blake2b.Sum512(nil) // Using nil for parent hash to simulate genesis block
	c1, err := marshalChange(&Config{
		Retention:  30 * 24 * time.Hour, // 30 days
		MaxStorage: 100 * 1024 * 1024,   // 100 MB
	})
	core.TestErr(t, err, "marshalChange failed: %v")

	c2, err := marshalChange(&AddKey{
		KeyId:         1,
		EncryptedKeys: map[security.PublicID][]byte{},
	})
	core.TestErr(t, err, "marshalChange failed: %v")

	block := Block{
		SnowID:       core.SnowID(),
		ParentHash:   hash[:],
		Timestamp:    core.Now(),
		BlockChanges: []BlockChange{c1, c2},
	}

	data, err := encodeBlock(id, block)
	core.TestErr(t, err, "createBlock failed: %v")

	block, err = decodeBlock(data)
	core.TestErr(t, err, "decodeBlock failed: %v")
	core.Assert(t, len(block.BlockChanges) == 2, "expected 2 changes, got %d", len(block.BlockChanges))
	core.Assert(t, block.Author == id.PublicIDMust(), "expected author to be %s, got %s", id.PublicIDMust(), block.Author)
	core.Assert(t, bytes.Equal(block.BlockChanges[0].Payload, c1.Payload), "expected first change to be settings, got %d", block.BlockChanges[0].Type)
	core.Assert(t, bytes.Equal(block.BlockChanges[1].Payload, c2.Payload), "expected second change to be addKey, got %d", block.BlockChanges[1].Type)
}
