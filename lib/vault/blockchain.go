package vault

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/crypto/blake2b"
	"gopkg.in/yaml.v2"
)

type BlockFlags byte

const (
	NewKey BlockFlags = 1 << iota
)

type Realm string

func (r Realm) String() string {
	return string(r)
}

const (
	Users Realm = "users" // Group for regular users
	Home  Realm = "home"  // Group for administrators
	All   Realm = "all"   // Group for public access
)

type Access byte
type Accesses map[security.PublicID]Access

const (
	Read Access = 1 << iota
	Write
	Admin
	ReadWrite      = Read + Write
	ReadWriteAdmin = Read + Write + Admin
)

var AccessLabels = []string{"", "R", "W", "RW", "A", "RA", "WA", "RWA"}

func (a Access) String() string {
	access := ""
	if a&Read != 0 {
		access += "R"
	}
	if a&Write != 0 {
		access += "W"
	}
	if a&Admin != 0 {
		access += "A"
	}
	return access
}

func (a *Accesses) String() string {
	return fmt.Sprintf("%v", *(*map[security.PublicID]Access)(a))
}

type BlockType uint16

const (
	BlockTypeSettings BlockType = iota
	BlockTypeChanges
)

type BlockChange struct {
	Type    ChangeType // Type of the change (AddAccess, ChangeKey, etc.)
	Payload []byte     // Marshalled change data
}

func (c BlockChange) String() string {
	d, _ := yaml.Marshal(c)
	return string(d)
}

type Block struct {
	SnowID       uint64            // Unique identifier for the block
	Signature    []byte            // Block's digital signature
	ParentHash   []byte            // Signature of the parent block
	Timestamp    time.Time         // Time of block creation
	Author       security.PublicID // Block creator's identity
	BlockChanges []BlockChange     // Block contents: list of changes
}

func (b Block) String() string {
	d, _ := yaml.Marshal(b)
	return string(d)
}

func encodeBlock(id security.PrivateID, block Block) ([]byte, error) {
	core.Start("%d changes, parent hash %x,", len(block.BlockChanges), block.ParentHash)

	h, err := blake2b.New512(nil)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot create hash", err)
	}

	for _, change := range block.BlockChanges {
		h.Write(change.Payload)
	}
	block.Author = id.PublicIDMust()

	h.Write(block.Author.Bytes())
	h.Write(block.ParentHash)
	h.Write(fmt.Appendf(nil, "%d", block.SnowID))
	//	h.Write([]byte(block.Timestamp.Format(time.RFC3339Nano)))
	h.Write(binary.BigEndian.AppendUint64(nil, block.SnowID))

	hash := h.Sum(nil)
	signature, err := security.Sign(id, hash)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot sign block", err)
	}
	block.Signature = signature

	data, err := msgpack.Marshal(block)
	if err != nil {
		return nil, core.Error(core.ParseError, "cannot marshal signed block", err)
	}
	data, err = core.GzipCompress(data)
	if err != nil {
		return nil, core.Error(core.EncodeError, "cannot compress signed block", err)
	}

	core.End("size %d, signature %x, hash %x, author %s", len(data), block.Signature, hash, block.Author)
	return data, nil
}

func decodeBlock(data []byte) (block Block, err error) {
	core.Start("size %d", len(data))

	// Decompress the block data
	data, err = core.GzipDecompress(data)
	if err != nil {
		return Block{}, core.Error(core.EncodeError, "cannot decompress block", err)
	}

	err = msgpack.Unmarshal(data, &block)
	if err != nil {
		core.Info("cannot unmarshal block")
		return Block{}, core.Error(core.ParseError, "cannot unmarshal block", err)
	}

	// Validate the signature
	if len(block.Signature) != security.SignatureSize {
		core.Info("invalid signature length: %d, expected: %d", len(block.Signature), security.SignatureSize)
		return Block{}, core.Error(core.AuthError, "invalid signature length: %d, expected: %d", len(block.Signature), security.SignatureSize)
	}

	h, err := blake2b.New512(nil)
	if err != nil {
		return Block{}, core.Error(core.GenericError, "cannot create hash", err)
	}

	for _, change := range block.BlockChanges {
		h.Write(change.Payload)
	}
	h.Write(block.Author.Bytes())
	h.Write(block.ParentHash)
	h.Write(fmt.Appendf(nil, "%d", block.SnowID))
	//	h.Write([]byte(block.Timestamp.Format(time.RFC3339Nano)))
	h.Write(binary.BigEndian.AppendUint64(nil, block.SnowID))

	hash := h.Sum(nil)
	if !security.Verify(block.Author, hash, block.Signature) {
		return Block{}, core.Error(core.AuthError, "invalid block signature: %x, author %s, hash %x", block.Signature, block.Author, hash)
	}

	core.Trace("decoded block with signature %x, parent hash %x, timestamp %s, author %s",
		block.Signature, block.ParentHash, block.Timestamp.Format(time.RFC3339Nano), block.Author)
	core.End("%d changes", len(block.BlockChanges))
	return block, nil
}

func (v *Vault) getLastBlockHash() ([]byte, error) {
	core.Start("")
	var lastHash []byte
	err := v.DB.QueryRow("GET_LAST_HASH", sqlx.Args{"vault": v.ID}, &lastHash)
	if err == sqlx.ErrNoRows {
		core.End("no last hash found, returning empty hash")
		return make([]byte, 64), nil
	}
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get last block hash", err)
	}
	core.End("hash %x", lastHash)
	return lastHash, nil
}

func (v *Vault) importBlockFromStorage(name string) (hash []byte, err error) {
	core.Start("name %s", name)
	blockPath := path.Join(v.Realm.String(), BlockChainFolder, name)

	now := core.Now()
	data, err := store.ReadFile(v.store, blockPath)
	if os.IsNotExist(err) {
		core.End("nothing to import")
		return nil, nil
	}
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot read block %s", blockPath, err)
	}

	block, err := decodeBlock(data)
	if err != nil {
		return nil, core.Error(core.ParseError, "cannot decode block %s", blockPath, err)
	}

	hash = core.BigHash(data)
	_, err = v.DB.Exec("SET_BLOCK", sqlx.Args{
		"vault":   v.ID,
		"name":    name,
		"showId":  block.SnowID,
		"hash":    hash,
		"payload": data,
	})
	if err != nil {
		return nil, core.Error(core.DbError, "cannot insert block %s into DB", blockPath, err)
	}

	for _, blockChange := range block.BlockChanges {
		c, err := unmarshalChange(blockChange)
		if err != nil {
			core.Error(core.ParseError, "cannot unmarshal change %v", blockChange, err)
			continue
		}
		err = c.Apply(v, block.Author)
		if err != nil {
			return nil, core.Error(core.GenericError, "cannot handle change %v", c, err)
		}
		core.Info("applied change %v from block %s author %x", c, blockPath, block.Author.Hash())
	}

	core.End("%d changes, hash %x, elapsed %v",
		len(block.BlockChanges), hash, core.Now().Sub(now))
	return hash, nil
}

func (v *Vault) importBlocksFromStorage() (hash []byte, err error) {
	core.Start("")
	hash, err = v.getLastBlockHash()
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get last block signature", err)
	}
	if hash == nil {
		hash = make([]byte, security.SignatureSize)
	}

	var cnt int
	for err == nil {
		var nextHash []byte
		name := base64.RawURLEncoding.EncodeToString(hash)

		nextHash, err = v.importBlockFromStorage(name)
		if err != nil {
			return nil, core.Error(core.GenericError, "cannot import block %s from store", name, err)
		}
		if nextHash == nil {
			core.End("%d blocks imported, last hash %x", cnt, hash)
			return hash, nil
		}
		cnt++
		hash = nextHash
	}
	return nil, core.Error(core.GenericError, "cannot import blocks from store", err)
}

func (v *Vault) exportBlocksToStorage(hash []byte) (retry bool, err error) {
	core.Start("hash %x", hash)

	blockChanges, err := v.getStagedChanges()
	if err != nil {
		return false, core.Error(core.DbError, "cannot get staged changes", err)
	}
	if len(blockChanges) == 0 {
		core.End("no staged changes to export")
		return true, nil // Nothing to export
	}

	block := Block{
		SnowID:       core.SnowID(),
		ParentHash:   hash,
		Timestamp:    core.Now(),
		BlockChanges: blockChanges,
	}

	payload, err := encodeBlock(v.UserSecret, block)
	if err != nil {
		return false, core.Error(core.EncodeError, "cannot encode block", err)
	}

	name := base64.RawURLEncoding.EncodeToString(hash)
	blockPath := path.Join(v.Realm.String(), BlockChainFolder, name)

	_, err = v.store.Stat(blockPath)
	if err == nil {
		core.End("block %s already exists, retrying", blockPath)
		return true, nil // Block already exists, retry
	}

	err = store.WriteFile(v.store, blockPath, payload)
	if err != nil {
		return true, core.Error(core.GenericError, "cannot write block %s", blockPath, err)
	}

	for i := 0; ; i++ {
		data, err := store.ReadFile(v.store, blockPath)
		if err != nil {
			return true, core.Error(core.GenericError, "cannot read block %s after writing", blockPath, err)
		}
		if bytes.Equal(payload, data) {
			break
		}
		if i >= 3 {
			core.End("data mismatch on %s after retries, original size %d, read size %d", blockPath, len(payload), len(data))
			return true, nil
		}
		core.Info("data mismatch on %s, retrying read %d", blockPath, i+1)
		time.Sleep(100 * time.Millisecond)
	}
	v.notifyChange(blockPath)

	for _, bc := range blockChanges {
		c, err := unmarshalChange(bc)
		if err != nil {
			return false, core.Error(core.ParseError, "cannot unmarshal change %v", bc, err)
		}

		err = c.Apply(v, v.UserID)
		if err != nil {
			return false, core.Error(core.GenericError, "cannot handle change %v", c, err)
		}
		core.Info("%s by %x in %s", c, v.UserID.Hash(), v.ID)
	}

	_, err = v.DB.Exec("SET_BLOCK", sqlx.Args{
		"vault":   v.ID,
		"name":    name,
		"showId":  block.SnowID,
		"hash":    core.BigHash(payload),
		"payload": payload,
	})
	if err != nil {
		return false, core.Error(core.DbError, "cannot insert block %s into DB", blockPath, err)
	}

	v.DB.Exec("DELETE_STAGED_CHANGES", sqlx.Args{"vault": v.ID})

	core.End("%d changes, file %s, hash %x", len(blockChanges), blockPath, hash)
	return true, nil
}

func (v *Vault) syncBlockChain() error {
	core.Start("")
	now := core.Now()
	v.blockChainMu.Lock()
	defer v.blockChainMu.Unlock()

	var success bool
	var cnt int
	for !success && cnt < 10 {
		lastHash, err := v.importBlocksFromStorage()
		if err != nil {
			return core.Error(core.GenericError, "cannot import blocks from store.", err)
		}

		success, err = v.exportBlocksToStorage(lastHash)
		if err != nil {
			return core.Error(core.GenericError, "cannot export changes to store.", err)
		}
	}

	if cnt == 10 {
		return core.Error(core.GenericError, "cannot sync blockchain after %d attempts", cnt)
	}

	core.End("done in %v", core.Now().Sub(now))
	return nil
}
