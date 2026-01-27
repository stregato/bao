// Package main exposes a thin, Go-friendly facade to the core bao APIs so Go
// callers can import github.com/stregato/bao/lib and reach the same primitives
// used by the cgo bindings without digging through subpackages.
package main

import (
	"github.com/stregato/bao/lib/replica"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/stregato/bao/lib/vault"
)

// Aliases to simplify downstream imports.
type (
	Vault   = vault.Vault
	Replica = replica.Replica
	Store   = store.Store
	Config  = vault.Config
	Realm   = vault.Realm
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

// CreateVault creates a new bao vault with the provided identity, backing store URL, and config.
func CreateVault(realm Realm, userPrivateID security.PrivateID, store store.Store, db *sqlx.DB, config Config) (*vault.Vault, error) {
	return vault.Create(realm, userPrivateID, store, db, config)
}

// OpenVault opens an existing bao vault with the provided identity, URL, and author.
func OpenVault(realm Realm, userPrivateID security.PrivateID, author security.PublicID, s store.Store, db *sqlx.DB) (*vault.Vault, error) {
	return vault.Open(realm, userPrivateID, author, s, db)
}

// OpenReplica opens a SQL-like layer for the specified bao vault and group.
func OpenReplica(s *vault.Vault, db *sqlx.DB) (*replica.Replica, error) {
	return replica.Open(s, db)
}

type StoreConfig = store.StoreConfig
