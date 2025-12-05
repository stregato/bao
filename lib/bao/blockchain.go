package bao

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
	"github.com/stregato/bao/lib/storage"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/crypto/blake2b"
	"gopkg.in/yaml.v2"
)

type BlockFlags byte

const (
	NewKey BlockFlags = 1 << iota
)

type Group string

func (g Group) String() string {
	return string(g)
}

const (
	Users  Group = "users"  // Group for regular users
	Admins Group = "admins" // Group for administrators
	Public Group = "public" // Group for public access
	SQL    Group = "sql"    // Group for SQL operations
)

type Access byte
type Accesses map[security.PublicID]Access

const (
	Read Access = 1 << iota
	Write
	Admin
	ReadWrite = Read + Write
)

var AccessLabels = []string{"", "R", "W", "A", "RW"}

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
		return nil, core.Errorw("cannot create hash", err)
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
		return nil, core.Errorw("cannot sign block", err)
	}
	block.Signature = signature

	data, err := msgpack.Marshal(block)
	if err != nil {
		return nil, core.Errorw("cannot marshal signed block", err)
	}
	data, err = core.GzipCompress(data)
	if err != nil {
		return nil, core.Errorw("cannot compress signed block", err)
	}

	core.End("size %d, signature %x, hash %x, author %s", len(data), block.Signature, hash, block.Author)
	return data, nil
}

func decodeBlock(data []byte) (block Block, err error) {
	core.Start("size %d", len(data))

	// Decompress the block data
	data, err = core.GzipDecompress(data)
	if err != nil {
		return Block{}, core.Errorw("cannot decompress block", err)
	}

	err = msgpack.Unmarshal(data, &block)
	if err != nil {
		core.Info("cannot unmarshal block")
		return Block{}, core.Errorw("cannot unmarshal block", err)
	}

	// Validate the signature
	if len(block.Signature) != security.SignatureSize {
		core.Info("invalid signature length: %d, expected: %d", len(block.Signature), security.SignatureSize)
		return Block{}, core.Errorw("invalid signature length: %d, expected: %d", len(block.Signature), security.SignatureSize)
	}

	h, err := blake2b.New512(nil)
	if err != nil {
		return Block{}, core.Errorw("cannot create hash", err)
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
		return Block{}, core.Errorw("invalid block signature: %x, author %s, hash %x", block.Signature, block.Author, hash)
	}

	core.Trace("decoded block with signature %x, parent hash %x, timestamp %s, author %s",
		block.Signature, block.ParentHash, block.Timestamp.Format(time.RFC3339Nano), block.Author)
	core.End("%d changes", len(block.BlockChanges))
	return block, nil
}

func (s *Bao) getLastBlockHash() ([]byte, error) {
	core.Start("")
	var lastHash []byte
	err := s.DB.QueryRow("GET_LAST_HASH", sqlx.Args{"store": s.Id}, &lastHash)
	if err == sqlx.ErrNoRows {
		core.End("no last hash found, returning empty hash")
		return make([]byte, 64), nil
	}
	if err != nil {
		return nil, core.Errorw("cannot get last block hash", err)
	}
	core.End("hash %x", lastHash)
	return lastHash, nil
}

func (s *Bao) importBlockFromStorage(name string) (hash []byte, err error) {
	core.Start("name %s", name)
	blockPath := path.Join(BlockChainFolder, name)

	now := core.Now()
	data, err := storage.ReadFile(s.store, blockPath)
	if os.IsNotExist(err) {
		core.End("nothing to import")
		return nil, nil
	}
	if err != nil {
		return nil, core.Errorw("cannot read block %s", blockPath, err)
	}

	block, err := decodeBlock(data)
	if err != nil {
		return nil, core.Errorw("cannot decode block %s", blockPath, err)
	}
	core.Info("importBlockFromStorage 1")

	hash = core.BigHash(data)
	_, err = s.DB.Exec("SET_BLOCK", sqlx.Args{
		"store":   s.Id,
		"name":    name,
		"showId":  block.SnowID,
		"hash":    hash,
		"payload": data,
	})
	if err != nil {
		return nil, core.Errorw("cannot insert block %s into DB", blockPath, err)
	}

	core.Info("importBlockFromStorage 2")

	for _, blockChange := range block.BlockChanges {
		core.Info("importBlockFromStorage 3")
		c, err := unmarshalChange(blockChange)
		if err != nil {
			core.Errorw("cannot unmarshal change %v", blockChange, err)
			continue
		}
		core.Info("importBlockFromStorage 4")
		err = c.Apply(s, block.Author)
		if err != nil {
			return nil, core.Errorw("cannot handle change %v", c, err)
		}
	}

	core.End("%d changes, hash %x, elapsed %v",
		len(block.BlockChanges), hash, core.Now().Sub(now))
	return hash, nil
}

func (s *Bao) importBlocksFromStorage() (hash []byte, err error) {
	core.Start("")
	hash, err = s.getLastBlockHash()
	if err != nil {
		return nil, core.Errorw("cannot get last block signature", err)
	}
	if hash == nil {
		hash = make([]byte, security.SignatureSize)
	}

	var cnt int
	for err == nil {
		var nextHash []byte
		name := base64.RawURLEncoding.EncodeToString(hash)

		nextHash, err = s.importBlockFromStorage(name)
		if err != nil {
			return nil, core.Errorw("cannot import block %s from storage", name, err)
		}
		if nextHash == nil {
			core.End("%d blocks imported, last hash %x", cnt, hash)
			return hash, nil
		}
		cnt++
		hash = nextHash
	}
	return nil, core.Errorw("cannot import blocks from storage", err)
}

func (s *Bao) exportBlocksToStorage(hash []byte) (retry bool, err error) {
	core.Start("hash %x", hash)

	blockChanges, err := s.getStagedChanges()
	if err != nil {
		return false, core.Errorw("cannot get staged changes", err)
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

	payload, err := encodeBlock(s.UserId, block)
	if err != nil {
		return false, core.Errorw("cannot encode block", err)
	}

	name := base64.RawURLEncoding.EncodeToString(hash)
	blockPath := path.Join(BlockChainFolder, name)

	_, err = s.store.Stat(blockPath)
	if err == nil {
		core.End("block %s already exists, retrying", blockPath)
		return true, nil // Block already exists, retry
	}

	err = storage.WriteFile(s.store, blockPath, payload)
	if err != nil {
		return true, core.Errorw("cannot write block %s", blockPath, err)
	}

	time.Sleep(time.Second)
	data, err := storage.ReadFile(s.store, blockPath)
	if err != nil {
		return true, core.Errorw("cannot read block %s after writing", blockPath, err)
	}

	if !bytes.Equal(payload, data) {
		core.End("data mismatch on %s, original size %d, read size %d", blockPath, len(payload), len(data))
		return true, nil
	}

	for _, bc := range blockChanges {
		c, err := unmarshalChange(bc)
		if err != nil {
			return false, core.Errorw("cannot unmarshal change %v", bc, err)
		}

		err = c.Apply(s, s.UserPublicId)
		if err != nil {
			return false, core.Errorw("cannot handle change %v", c, err)
		}
	}

	_, err = s.DB.Exec("SET_BLOCK", sqlx.Args{
		"store":   s.Id,
		"name":    name,
		"showId":  block.SnowID,
		"hash":    core.BigHash(payload),
		"payload": payload,
	})
	if err != nil {
		return false, core.Errorw("cannot insert block %s into DB", blockPath, err)
	}

	s.DB.Exec("DELETE_STAGED_CHANGES", sqlx.Args{"store": s.Id})

	core.End("%d changes, file %s, hash %x", len(blockChanges), blockPath, hash)
	return true, nil
}

func (s *Bao) syncBlockChain() error {
	core.Start("")
	now := core.Now()
	if s.store == nil {
		var err error
		s.store, err = storage.Open(s.Url)
		if err != nil {
			return core.Errorw("cannot open store with connection URL %s", s.Url, err)
		}
	}

	s.blockChainMu.Lock()
	defer s.blockChainMu.Unlock()

	var success bool
	var cnt int
	for !success && cnt < 10 {
		lastHash, err := s.importBlocksFromStorage()
		if err != nil {
			return core.Errorw("cannot import blocks from storage", err)
		}

		success, err = s.exportBlocksToStorage(lastHash)
		if err != nil {
			return core.Errorw("cannot export changes to storage", err)
		}
	}

	if cnt == 10 {
		return core.Errorw("cannot sync blockchain after %d attempts", cnt)
	}

	core.End("done in %v", core.Now().Sub(now))
	return nil
}
