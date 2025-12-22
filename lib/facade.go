// Package main exposes a thin, Go-friendly facade to the core bao APIs so Go
// callers can import github.com/stregato/bao/lib and reach the same primitives
// used by the cgo bindings without digging through subpackages.
package main

import (
	"github.com/stregato/bao/lib/bao"
	"github.com/stregato/bao/lib/ql"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

// Aliases to simplify downstream imports.
type (
	Bao   = bao.Bao
	BaoQL = ql.BaoQL
)

// NewPrivateID generates a new identity containing the signing and encryption keys.
func NewPrivateID() (security.PrivateID, error) {
	return security.NewPrivateID()
}

// PublicID derives the public half of a PrivateID.
func PublicID(id security.PrivateID) (security.PublicID, error) {
	return id.PublicID()
}

// DecodeID splits a composite ID into its encryption and signing keys.
func DecodeID(id string) ([]byte, []byte, error) {
	return security.DecodeID(id)
}

// ECEncrypt encrypts data using the provided public identity.
func ECEncrypt(id security.PublicID, data []byte) ([]byte, error) {
	return security.EcEncrypt(id, data)
}

// ECDecrypt decrypts data using the provided private identity.
func ECDecrypt(id security.PrivateID, data []byte) ([]byte, error) {
	return security.EcDecrypt(id, data)
}

// AESEncrypt encrypts bytes with a symmetric key and nonce.
func AESEncrypt(key []byte, nonce []byte, data []byte) ([]byte, error) {
	return security.AESEncrypt(key, nonce, data)
}

// AESDecrypt decrypts bytes produced by AESEncrypt.
func AESDecrypt(key []byte, nonce []byte, cipherdata []byte) ([]byte, error) {
	return security.AESDecrypt(key, nonce, cipherdata)
}

// OpenDB opens a database connection with optional bootstrap DDL.
func OpenDB(driverName, dataSource, ddl string) (*sqlx.DB, error) {
	return sqlx.Open(driverName, dataSource, ddl)
}

// CreateBao creates a new bao vault with the provided identity, backing store URL, and config.
func CreateBao(db *sqlx.DB, id security.PrivateID, c storage.StoreConfig, cfg bao.Config) (*bao.Bao, error) {
	return bao.Create(db, id, c, cfg)
}

// OpenBao opens an existing bao vault with the provided identity, URL, and author.
func OpenBao(db *sqlx.DB, id security.PrivateID, c storage.StoreConfig, author security.PublicID) (*bao.Bao, error) {
	return bao.Open(db, id, c, author)
}

// SQL attaches a BaoQL layer for replicated SQL over the vault.
func SQL(s *bao.Bao, group bao.Group, db *sqlx.DB) (*ql.BaoQL, error) {
	return ql.SQL(s, group, db)
}

type StoreConfig = storage.StoreConfig
