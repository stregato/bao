//go:build !js

package main

/*
#include "cfunc.h"
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <stdio.h>

*/
import "C"
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/mailbox"
	"github.com/stregato/bao/lib/replica"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/stregato/bao/lib/vault"
	"gopkg.in/yaml.v2"
)

func cResult(v any, hnd int64, err error) C.Result {
	var val []byte

	if err != nil {
		errMsg, _ := json.Marshal(err)
		return C.Result{nil, 0, C.longlong(hnd), C.CString(string(errMsg))}
	}
	if v == nil {
		return C.Result{nil, 0, C.longlong(hnd), nil}
	}

	val, ok := v.([]byte)
	if !ok {
		val, err = json.Marshal(v)
		if err != nil {
			logrus.Errorf("cannot marshal result %v: %v", v, err)
			return C.Result{nil, 0, C.longlong(hnd), C.CString(err.Error())}
		}
	}
	if err != nil {
		errMsg, _ := json.Marshal(err)
		return C.Result{nil, 0, C.longlong(hnd), C.CString(string(errMsg))}
	}
	// Allocate memory in the C heap
	len := C.size_t(len(val))
	ptr := C.malloc(len)
	if ptr == nil {
		core.LogError("memory allocation failed")
		return C.Result{nil, 0, C.longlong(hnd), C.CString("memory allocation failed")}
	}
	// Copy data from Go slice to C heap
	C.memcpy(ptr, unsafe.Pointer(&val[0]), len)
	return C.Result{ptr, len, C.longlong(hnd), nil}
}

func cInput(err error, i *C.char, v any) error {
	if err != nil {
		return err
	}
	data := C.GoString(i)
	err = json.Unmarshal([]byte(data), v)
	if err != nil {
		return core.Error(core.ParseError, "failed to unmarshal input - %v: %s", err, data)
	}
	return nil
}

var (
	stores   core.Registry[store.Store]
	dbs      core.Registry[*sqlx.DB]
	vaults   core.Registry[*vault.Vault]
	replicas core.Registry[*replica.Replica]
	rows     core.Registry[*sqlx.RowsX]
)

// bao_setLogLevel sets the log level for the vault library. Possible values are: trace, debug, info, warn, error, fatal, panic.
//
//export bao_setLogLevel
func bao_setLogLevel(level *C.char) C.Result {
	core.Start("level %s", C.GoString(level))
	switch C.GoString(level) {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	}
	core.End("level %s", C.GoString(level))
	return cResult(nil, 0, nil)
}

// bao_core_setHttpLog sets the HTTP endpoint for log messages.
//
//export bao_core_setHttpLog
func bao_core_setHttpLog(url *C.char) C.Result {
	core.Start("url %s", C.GoString(url))
	core.SetHttpLog(C.GoString(url))
	core.End("url %s", C.GoString(url))
	return cResult(nil, 0, nil)
}

// bao_core_getRecentLog retrieves the most recent log messages up to the specified number.
//
//export bao_core_getRecentLog
func bao_core_getRecentLog(n C.int) C.Result {
	core.Start("n %d", n)
	logs := core.GetRecentLog(int(n))
	core.End("n %d", n)
	return cResult(logs, 0, nil)
}

//export bao_test
func bao_test() C.Result {
	core.Start("")
	core.End("")
	return cResult(nil, 0, nil)
}

// bao_security_newPrivateID creates a new identity with the specified nick. An identity is a key pair used for encryption and signing, and a nick name for human readable identification.
// An identity is made of two fields ID and Private. ID is a concatenation of the nick name and the public key of the key pair. Private is the private key of the key pair.
//
//export bao_security_newPrivateID
func bao_security_newPrivateID() C.Result {
	core.Start("")
	core.TimeTrack()
	identity, err := security.NewPrivateID()
	if err != nil {
		core.LogError("cannot generate private ID", err)
		core.End("failed to generate private ID")
		return cResult(nil, 0, err)
	}
	core.End("generated new private ID")
	return cResult(identity, 0, nil)
}

// bao_security_publicID returns the public ID of the specified identity.
//
//export bao_security_publicID
func bao_security_publicID(privateID *C.char) C.Result {
	privID := C.GoString(privateID)
	core.Start("private ID len %d", len(privID))
	core.TimeTrack()
	id, err := security.PrivateID(privID).PublicID()
	if err != nil {
		core.LogError("cannot derive public ID from provided private ID", err)
		core.End("failed to derive public ID")
		return cResult(nil, 0, err)
	}
	core.End("public ID derived")
	return cResult(id, 0, nil)
}

// bao_security_newKeyPair creates a new key pair and returns the public and private IDs.
//
//export bao_security_newKeyPair
func bao_security_newKeyPair() C.Result {
	core.Start("")
	core.TimeTrack()
	publicID, privateID, err := security.NewKeyPair()
	if err != nil {
		core.LogError("cannot generate new key pair", err)
		core.End("failed to generate new key pair")
		return cResult(nil, 0, err)
	}
	core.End("generated new key pair")
	return cResult(map[string]string{"publicID": string(publicID), "privateID": string(privateID)}, 0, nil)
}

// bao_security_ecEncrypt encrypts the specified plaintext using the provided identity.
//
//export bao_security_ecEncrypt
func bao_security_ecEncrypt(id *C.char, plainData C.Data) C.Result {
	idStr := C.GoString(id)
	core.Start("id len %d", len(idStr))
	core.TimeTrack()

	data := C.GoBytes(plainData.ptr, C.int(plainData.len))

	cipherData, err := security.EcEncrypt(security.PublicID(idStr), data)
	if err != nil {
		core.LogError("cannot encrypt plaintext with provided public ID", err)
		core.End("failed for id len %d", len(idStr))
		return cResult(nil, 0, err)
	}
	core.End("id len %d", len(idStr))
	return cResult(cipherData, 0, nil)
}

// bao_security_ecDecrypt decrypts the specified ciphertext using the provided identity.
//
//export bao_security_ecDecrypt
func bao_security_ecDecrypt(id *C.char, cipherData C.Data) C.Result {
	idStr := C.GoString(id)
	core.Start("id len %d", len(idStr))
	core.TimeTrack()
	data := C.GoBytes(cipherData.ptr, C.int(cipherData.len))
	plainData, err := security.EcDecrypt(security.PrivateID(idStr), data)
	if err != nil {
		core.LogError("cannot decrypt ciphertext with provided private ID", err)
		core.End("failed for id len %d", len(idStr))
		return cResult(nil, 0, err)
	}
	core.End("id len %d", len(idStr))
	return cResult(plainData, 0, nil)
}

// bao_security_aesEncrypt encrypts the specified plaintext using the provided key/nonce.
//
//export bao_security_aesEncrypt
func bao_security_aesEncrypt(key *C.char, nonceData, plainData C.Data) C.Result {
	keyStr := C.GoString(key)
	core.Start("key len %d", len(keyStr))
	core.TimeTrack()
	nonce := C.GoBytes(nonceData.ptr, C.int(nonceData.len))
	data := C.GoBytes(plainData.ptr, C.int(plainData.len))
	cipherText, err := security.AESEncrypt([]byte(keyStr), nonce, data)
	if err != nil {
		core.LogError("cannot encrypt plaintext with provided key (len %d) and nonce (%d bytes)", len(keyStr), len(nonce), err)
		core.End("failed for key len %d", len(keyStr))
		return cResult(nil, 0, err)
	}
	core.End("key len %d", len(keyStr))
	return cResult(cipherText, 0, nil)
}

// bao_security_aesDecrypt decrypts the specified ciphertext using the provided key/nonce.
//
//export bao_security_aesDecrypt
func bao_security_aesDecrypt(key *C.char, nonceData, cipherData C.Data) C.Result {
	keyStr := C.GoString(key)
	core.Start("key len %d", len(keyStr))
	core.TimeTrack()
	nonce := C.GoBytes(nonceData.ptr, C.int(nonceData.len))
	data := C.GoBytes(cipherData.ptr, C.int(cipherData.len))
	plainText, err := security.AESDecrypt([]byte(keyStr), nonce, data)
	if err != nil {
		core.LogError("cannot decrypt ciphertext with provided key (len %d) and nonce (%d bytes)", len(keyStr), len(nonce), err)
		core.End("failed for key len %d", len(keyStr))
		return cResult(nil, 0, err)
	}
	core.End("key len %d", len(keyStr))
	return cResult(plainText, 0, nil)
}

// bao_security_decodePublicID decodes the specified public identity into the crypt key and the sign key.
//
//export bao_security_decodePublicID
func bao_security_decodePublicID(idC *C.char) C.Result {
	id := C.GoString(idC)
	core.Start("id len %d", len(id))
	core.TimeTrack()
	cryptKey, signKey, err := security.PublicID(id).Decode()
	if err != nil {
		core.LogError("cannot decode ID", err)
		core.End("failed to decode ID")
		return cResult(nil, 0, err)
	}

	cryptKey64 := base64.URLEncoding.EncodeToString(cryptKey)
	signKey64 := base64.URLEncoding.EncodeToString(signKey)

	core.End("decoded ID into key components")
	return cResult(map[string]string{"cryptKey": cryptKey64, "signKey": signKey64}, 0, err)
}

// bao_security_decodePrivateID decodes the specified private identity into the crypt key and the sign key.
//
//export bao_security_decodePrivateID
func bao_security_decodePrivateID(idC *C.char) C.Result {
	id := C.GoString(idC)
	core.Start("id len %d", len(id))
	core.TimeTrack()
	cryptKey, signKey, err := security.PrivateID(id).Decode()
	if err != nil {
		core.LogError("cannot decode ID", err)
		core.End("failed to decode ID")
		return cResult(nil, 0, err)
	}

	cryptKey64 := base64.URLEncoding.EncodeToString(cryptKey)
	signKey64 := base64.URLEncoding.EncodeToString(signKey)

	core.End("decoded ID into key components")
	return cResult(map[string]string{"cryptKey": cryptKey64, "signKey": signKey64}, 0, err)
}

// bao_db_open opens a new database connection to the specified URL.Bao library requires a database connection to store safe and file system data. The function returns a handle to the database connection.
//
//export bao_db_open
func bao_db_open(driverName, dataSourceName, ddl *C.char) C.Result {
	core.Start("driver %s", C.GoString(driverName))
	var db *sqlx.DB
	var err error

	core.TimeTrack()

	db, err = sqlx.Open(C.GoString(driverName), C.GoString(dataSourceName), C.GoString(ddl))
	if err != nil {
		core.LogError("cannot open db with url %s", C.GoString(dataSourceName), err)
		return cResult(nil, 0, err)
	}

	core.End("url %s", C.GoString(dataSourceName))
	return cResult(db, dbs.Add(db), err)
}

// bao_db_close closes the specified database connection.
//
//export bao_db_close
func bao_db_close(dbH C.longlong) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH, err)
		return cResult(nil, 0, err)
	}
	d.Close()
	dbs.Remove(int64(dbH))
	core.End("handle %d", dbH)
	return cResult(nil, 0, nil)
}

// bao_db_query executes the specified query on the database connection identified by the given handle.
//
//export bao_db_query
func bao_db_query(dbH C.longlong, queryC *C.char, argsC *C.char) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH, err)
		return cResult(nil, 0, err)
	}

	query := C.GoString(queryC)

	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}

	res, err := d.Query(query, args)
	core.End("query %s", query)
	return cResult(nil, rows.Add(&res), err)
}

// bao_db_exec executes the specified query on the database connection identified by the given handle and returns a single row.
//
//export bao_db_exec
func bao_db_exec(dbH C.longlong, queryC *C.char, argsC *C.char) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH, err)
		return cResult(nil, 0, err)
	}
	query := C.GoString(queryC)
	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}

	_, err = d.Exec(query, args)
	core.End("query %s", query)
	return cResult(nil, 0, err)
}

// bao_db_fetch fetches rows from the specified query on the database connection identified by the given handle.
//
//export bao_db_fetch
func bao_db_fetch(dbH C.longlong, queryC *C.char, argsC *C.char, maxRows C.int) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH, err)
		return cResult(nil, 0, err)
	}
	query := C.GoString(queryC)
	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}

	rows, err := d.Fetch(query, args, int(maxRows))
	if err != nil {
		core.LogError("cannot fetch rows for query %s", query, err)
		return cResult(nil, 0, err)
	}
	core.End("query %s", query)
	return cResult(rows, 0, nil)
}

// bao_db_fetch_one fetches a single row from the specified query on the database connection identified by the given handle.
//
//export bao_db_fetch_one
func bao_db_fetch_one(dbH C.longlong, queryC *C.char, argsC *C.char) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH, err)
		return cResult(nil, 0, err)
	}
	query := C.GoString(queryC)
	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}
	row, err := d.FetchOne(query, args)
	if err != nil {
		core.LogError("cannot fetch one row for query %s", query, err)
		return cResult(nil, 0, err)
	}
	core.End("query %s", query)
	return cResult(row, 0, nil)
}

// bao_store_open opens a store.backend with the specified configuration. The function returns a handle to the store.backend.
//
//export bao_store_open
func bao_store_open(configC *C.char) C.Result {
	core.Start("store config %s", C.GoString(configC))
	core.TimeTrack()
	var storeConfig store.StoreConfig
	err := cInput(nil, configC, &storeConfig)
	if err != nil {
		core.LogError("cannot unmarshal store config %s", C.GoString(configC), err)
		return cResult(nil, 0, err)
	}

	store, err := store.Open(storeConfig)
	if err != nil {
		core.LogError("cannot open store %s", storeConfig.Id, err)
		return cResult(nil, 0, err)
	}

	stores.Add(store)
	core.End("opened store %s", storeConfig.Id)
	return cResult(store, stores.Add(store), nil)
}

// bao_store_close closes the specified store.backend.
//
//export bao_store_close
func bao_store_close(storeH C.longlong) C.Result {
	core.Start("handle %d", storeH)
	core.TimeTrack()
	s, err := stores.Get(int64(storeH))
	if err != nil {
		core.LogError("cannot get store with handle %d", storeH, err)
		return cResult(nil, 0, err)
	}

	err = s.Close()
	if err != nil {
		core.LogError("cannot close store with handle %d", storeH, err)
		return cResult(nil, 0, err)
	}

	stores.Remove(int64(storeH))
	core.End("handle %d", storeH)
	return cResult(nil, 0, nil)
}

// bao_store_readDir reads the contents of the specified directory from the store.backend.
//
//export bao_store_readDir
func bao_store_readDir(storeH C.longlong, dirC *C.char, filterC *C.char) C.Result {
	core.Start("handle %d", storeH)
	core.TimeTrack()
	s, err := stores.Get(int64(storeH))
	if err != nil {
		core.LogError("cannot get store with handle %d", storeH, err)
		return cResult(nil, 0, err)
	}

	dir := C.GoString(dirC)

	var filter store.Filter
	err = cInput(err, filterC, &filter)
	if err != nil {
		core.LogError("cannot unmarshal filter %s", C.GoString(filterC), err)
		return cResult(nil, 0, err)
	}

	entries, err := s.ReadDir(dir, filter)
	if err != nil {
		core.LogError("cannot read dir %s from store with handle %d", dir, storeH, err)
		return cResult(nil, 0, err)
	}

	core.End("handle %d", storeH)
	return cResult(entries, 0, nil)
}

// bao_store_stat retrieves the metadata of the specified path from the store.backend.
//
//export bao_store_stat
func bao_store_stat(storeH C.longlong, pathC *C.char) C.Result {
	core.Start("handle %d", storeH)
	core.TimeTrack()
	s, err := stores.Get(int64(storeH))
	if err != nil {
		core.LogError("cannot get store with handle %d", storeH, err)
		return cResult(nil, 0, err)
	}

	path := C.GoString(pathC)

	info, err := s.Stat(path)
	if err != nil {
		core.LogError("cannot stat path %s from store with handle %d", path, storeH, err)
		return cResult(nil, 0, err)
	}

	core.End("handle %d", storeH)
	return cResult(info, 0, nil)
}

// bao_store_delete deletes the specified path from the store.backend.
//
//export bao_store_delete
func bao_store_delete(storeH C.longlong, pathC *C.char) C.Result {
	core.Start("handle %d", storeH)
	core.TimeTrack()
	s, err := stores.Get(int64(storeH))
	if err != nil {
		core.LogError("cannot get store with handle %d", storeH, err)
		return cResult(nil, 0, err)
	}

	path := C.GoString(pathC)

	err = s.Delete(path)
	if err != nil {
		core.LogError("cannot delete path %s from store with handle %d", path, storeH, err)
		return cResult(nil, 0, err)
	}

	core.End("handle %d", storeH)
	return cResult(nil, 0, nil)
}

// bao_vault_create creates a new vault with the specified identity, URL and configuration. A vault is a secure store.for keys and files. The function returns a handle to the bao.
//
//export bao_vault_create
func bao_vault_create(realmC *C.char, userPrivateID *C.char, storeH C.longlong, dbH C.longlong, configC *C.char) C.Result {
	core.Start("db handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH)
		return cResult(nil, 0, err)
	}

	store, err := stores.Get(int64(storeH))
	if err != nil {
		core.LogError("cannot get store with handle %d", storeH)
		return cResult(nil, 0, err)
	}

	var config vault.Config
	err = cInput(err, configC, &config)
	if err != nil {
		core.LogError("cannot unmarshal settings %s", C.GoString(configC))
		return cResult(nil, 0, err)
	}

	me := C.GoString(userPrivateID)
	realm := C.GoString(realmC)
	s, err := vault.Create(vault.Realm(realm), security.PrivateID(me), store, d, config)
	if err != nil {
		core.LogError("cannot create vault for store %s", store.ID(), err)
		return cResult(nil, 0, err)
	}

	core.End("created vault for store %s", store.ID())
	return cResult(s, vaults.Add(s), err)
}

// bao_vault_open opens an existing vault with the specified identity, author and URL. The function returns a handle to the bao.
//
//export bao_vault_open
func bao_vault_open(realmC *C.char, meC *C.char, authorC *C.char, storeH C.longlong, dbH C.longlong) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()

	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH, err)
		return cResult(nil, 0, err)
	}

	store, err := stores.Get(int64(storeH))
	if err != nil {
		core.LogError("cannot get store with handle %d", storeH, err)
		return cResult(nil, 0, err)
	}

	me := C.GoString(meC)
	author := C.GoString(authorC)
	realm := C.GoString(realmC)
	s, err := vault.Open(vault.Realm(realm), security.PrivateID(me), security.PublicID(author), store, d)
	if err != nil {
		core.LogError("cannot open vault for store %s", store.ID(), err)
		return cResult(nil, 0, err)
	}

	core.End("vault open for store %s", store.ID())
	return cResult(s, vaults.Add(s), nil)
}

// bao_vault_close closes the specified safe.
//
//export bao_vault_close
func bao_vault_close(sH C.longlong) C.Result {
	core.Start("handle %d", sH)
	core.TimeTrack()
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	s.Close()
	vaults.Remove(int64(sH))
	core.End("handle %d", sH)
	return cResult(nil, 0, nil)
}

// bao_vault_syncAccess sets and optionally flushes access rights for the specified vault.
//
//export bao_vault_syncAccess
func bao_vault_syncAccess(sH C.longlong, optionsC C.int, changesC *C.char) C.Result {
	core.Start("handle %d", sH)
	core.TimeTrack()
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	var changes []vault.AccessChange
	changesJSON := C.GoString(changesC)
	if err := json.Unmarshal([]byte(changesJSON), &changes); err != nil {
		core.LogError("cannot unmarshal access change payload", err)
		return cResult(nil, 0, err)
	}

	err = s.SyncAccess(vault.IOOption(optionsC), changes...)
	if err != nil {
		core.LogError("cannot synchronize access changes", err)
		return cResult(nil, 0, err)
	}
	core.End("handled %d access changes", len(changes))
	return cResult(nil, 0, nil)
}

// bao_vault_getAccesses returns the users and access rights for the specified group.
//
//export bao_vault_getAccesses
func bao_vault_getAccesses(vH C.longlong) C.Result {
	core.Start("handle %d", vH)
	core.TimeTrack()
	v, err := vaults.Get(int64(vH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", vH, err)
		return cResult(nil, 0, err)
	}

	a, err := v.GetAccesses()
	if err != nil {
		core.LogError("cannot get access for vault", err)
		return cResult(nil, 0, err)
	}
	core.End("vault access retrieved")
	return cResult(a, 0, nil)
}

// bao_vault_getAccess returns the access rights for the specified user in the bao.
//
//export bao_vault_getAccess
func bao_vault_getAccess(vH C.longlong, userC *C.char) C.Result {
	core.Start("handle %d", vH)
	core.TimeTrack()
	v, err := vaults.Get(int64(vH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", vH, err)
		return cResult(nil, 0, err)
	}

	user := C.GoString(userC)
	access, err := v.GetAccess(security.PublicID(user))
	if err != nil {
		core.LogError("cannot get access for user %s in vault", user, err)
		return cResult(nil, 0, err)
	}
	core.End("vault access retrieved for user %s", user)
	return cResult(access, 0, nil)
}

// bao_vault_sync synchronizes the file system for the specified groups in the bao.
//
//export bao_vault_sync
func bao_vault_sync(vH C.longlong) C.Result {
	var groups []vault.Realm

	core.TimeTrack()
	core.Start("handle %d", vH)
	s, err := vaults.Get(int64(vH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", vH, err)
		return cResult(nil, 0, err)
	}

	files, err := s.Sync()
	if err != nil {
		core.LogError("cannot synchronize groups %v in vault %d", groups, vH, err)
		return cResult(nil, 0, err)
	}

	core.End("bao_sync successful for vault %d with groups: %v", vH, groups)
	return cResult(files, 0, nil)
}

// bao_vault_waitFiles completes the read and write operations for the specified files in the bao.
// Returns the JSON-encoded array of files that completed I/O operations.
//
//export bao_vault_waitFiles
func bao_vault_waitFiles(sH C.longlong, timeoutMs C.longlong, fileIdsC *C.char) C.Result {
	var fileIds []vault.FileId

	core.TimeTrack()
	core.Start("called with sH: %d, timeout: %dms", sH, timeoutMs)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	if fileIdsC != nil {
		err = cInput(err, fileIdsC, &fileIds)
		if err != nil {
			logrus.Errorf("cannot unmarshal file IDs %s: %v", C.GoString(fileIdsC), err)
			return cResult(nil, 0, err)
		}
	}

	ctx := context.Background()
	if timeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}

	files, err := s.WaitFiles(ctx, fileIds...)
	if err != nil {
		logrus.Errorf("cannot synchronize vault %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	core.End("bao_sync successful for vault %d with %d files", sH, len(files))
	return cResult(files, 0, nil)
}

// bao_vault_setAttribute sets an attribute for the current user
//
//export bao_vault_setAttribute
func bao_vault_setAttribute(sH C.longlong, options C.int, nameC, valueC *C.char) C.Result {
	core.TimeTrack()
	core.Start("handle %d", sH)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	name := C.GoString(nameC)
	value := C.GoString(valueC)

	err = s.SetAttribute(vault.IOOption(options), name, value)
	if err != nil {
		core.LogError("cannot set attribute %s to %s in vault %d", name, value, sH, err)
		return cResult(nil, 0, err)
	}
	core.End("set attribute %s to %s in vault %d", name, value, sH)
	return cResult(nil, 0, nil)
}

// bao_vault_getAttribute gets an attribute for the specified user
//
//export bao_vault_getAttribute
func bao_vault_getAttribute(sH C.longlong, nameC, authorC *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s, author: %s", sH, C.GoString(nameC), C.GoString(authorC))
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	name := C.GoString(nameC)
	author := C.GoString(authorC)
	authorLen := len(author)

	value, err := s.GetAttribute(name, security.PublicID(author))
	if err != nil {
		core.LogError("cannot get attribute '%s' for provided user (len %d) in vault %d", name, authorLen, sH, err)
		return cResult(nil, 0, err)
	}
	core.End("got attribute '%s' for user len %d in vault %d", name, authorLen, sH)
	return cResult(value, 0, nil)
}

// bao_vault_getAttributes gets all attributes for the specified user
//
//export bao_vault_getAttributes
func bao_vault_getAttributes(sH C.longlong, authorC *C.char) C.Result {
	core.TimeTrack()
	core.Start("handle %d", sH)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	author := C.GoString(authorC)
	authorLen := len(author)

	attrs, err := s.GetAttributes(security.PublicID(author))
	if err != nil {
		core.LogError("cannot get attributes for provided user (len %d) in vault %d", authorLen, sH, err)
		return cResult(nil, 0, err)
	}
	core.End("got attributes for user len %d in vault %d", authorLen, sH)
	return cResult(attrs, 0, nil)
}

// bao_vault_readDir reads the specified directory from the bao.
//
//export bao_vault_readDir
func bao_vault_readDir(sH C.longlong, dir *C.char, after, fromId C.longlong, limit C.int) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dir: %s, after: %d, fromId: %d, limit: %d", sH, C.GoString(dir), after, fromId, limit)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	list, err := s.ReadDir(C.GoString(dir), time.Unix(int64(after), 0), int64(fromId), int(limit))
	core.End("bao_readDir returning: %v", list)

	return cResult(list, 0, err)
}

// bao_stat returns the file information for the specified file in the bao.
//
//export bao_vault_stat
func bao_vault_stat(sH C.longlong, name *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s", sH, C.GoString(name))
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	info, err := s.Stat(C.GoString(name))
	if os.IsNotExist(err) {
		return cResult(nil, 0, err)
	}
	if err != nil {
		core.LogError("cannot get file info for %s in vault %d", C.GoString(name), sH, err)
		return cResult(nil, 0, err)
	}
	core.End("successful statistic for file %s in vault %d", C.GoString(name), sH)
	return cResult(info, 0, err)
}

// bao_getAuthor returns the author of the specified file.
//
// bao_waitFiles waits for pending I/O for the specified files in the bao.
//
//export bao_vault_getAuthor
func bao_vault_getAuthor(sH C.longlong, name *C.char) C.Result {
	core.TimeTrack()
	core.Start("bao_vault_getAuthor called with sH: %d, name: %s", sH, C.GoString(name))
	s, err := vaults.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	author, err := s.GetAuthor(C.GoString(name))
	core.End("successful author retrieval for file %s in vault %d", C.GoString(name), sH)
	return cResult(author, 0, err)
}

// bao_vault_read reads the specified file from the bao.
//
//export bao_vault_read
func bao_vault_read(sH C.longlong, name, destC *C.char, options C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s, dest: %s, options: %d", sH, C.GoString(name), C.GoString(destC), options)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	dest := C.GoString(destC)
	file, err := s.Read(C.GoString(name), dest, vault.IOOption(options), nil)
	if err != nil {
		core.LogError("cannot read file %s from vault %d", C.GoString(name), sH, err)
		return cResult(nil, 0, err)
	}

	core.End("successfully read file %s from vault %d to %s", C.GoString(name), sH, C.GoString(destC))
	return cResult(file, 0, err)
}

// bao_vault_write writes the specified file to the bao.
//
//export bao_vault_write
func bao_vault_write(sH C.longlong, destC, sourceC *C.char, attrsC C.Data, options C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dest: %s, source: %s, options: %d", sH, C.GoString(destC), C.GoString(sourceC), options)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	dest := C.GoString(destC)
	source := C.GoString(sourceC)

	attrs := C.GoBytes(unsafe.Pointer(attrsC.ptr), C.int(attrsC.len))

	file, err := s.Write(dest, source, attrs, vault.IOOption(options), nil)
	if err != nil {
		core.LogError("cannot write file %s to vault %d, src %s: %v", dest, sH, source, err)
		return cResult(nil, 0, err)
	}

	core.End("successfully wrote file %s to vault %d, src %s", dest, sH, source)
	return cResult(file, 0, nil)
}

// bao_vault_delete deletes the specified file from the bao.
//
//export bao_vault_delete
func bao_vault_delete(sH C.longlong, nameC *C.char, options int) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s", sH, C.GoString(nameC))
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	name := C.GoString(nameC)
	err = s.Delete(name, vault.IOOption(options))
	if err != nil {
		core.LogError("cannot delete file %s from vault %d", name, sH, err)
		return cResult(nil, 0, err)
	}

	core.End("successfully deleted file %s from vault %d", name, sH)
	return cResult(nil, 0, nil)
}

// bao_vault_versions returns the versions of the specified file in the bao.
//
//export bao_vault_versions
func bao_vault_versions(sH C.longlong, nameC *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s", sH, C.GoString(nameC))
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	name := C.GoString(nameC)
	versions, err := s.Versions(name)
	if err != nil {
		core.LogError("cannot get versions for file %s in vault %d", name, sH, err)
		return cResult(nil, 0, err)
	}

	core.End("retrieved %d versions for file %s in vault %d", len(versions), name, sH)
	return cResult(versions, 0, nil)
}

// bao_vault_allocatedSize returns the allocated size of the specified bao.
//
//export bao_vault_allocatedSize
func bao_vault_allocatedSize(sH C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d", sH)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	size := s.AllocatedSize()
	core.End("allocated size for vault %d: %d", sH, size)
	return cResult(size, 0, nil)
}

// bao_replica_open returns a SQL like layer for the specified bao. The layer is used to execute SQL like commands on the vault data.
//
//export bao_replica_open
func bao_replica_open(sH C.longlong, dbH C.int) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dbH: %d", sH, dbH)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	db, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d", dbH, err)
		return cResult(nil, 0, err)
	}

	dt, err := replica.Open(s, db)
	if err != nil {
		core.LogError("cannot create sql layer for vault %d", sH, err)
		return cResult(nil, 0, err)
	}
	core.End("sql layer for vault %d created", sH)
	return cResult(dt, replicas.Add(dt), err)
}

// bao_replica_exec executes the specified SQL like command on the specified data table.
//
//export bao_replica_exec
func bao_replica_exec(dtH C.longlong, keyC, argsC *C.char) C.Result {
	core.TimeTrack()

	key := C.GoString(keyC)
	core.Start("dtH: %d, key: %s", dtH, key)

	dt, err := replicas.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	_, err = dt.Exec(C.GoString(keyC), args)
	if err != nil {
		core.LogError("cannot execute query %s with args %v", key, map[string]any(args), err)
	}
	core.End("")
	return cResult(nil, 0, err)
}

// bao_replica_query executes the specified SQL like query on the specified data table.
//
//export bao_replica_query
func bao_replica_query(dtH C.longlong, keyC, argsC *C.char) C.Result {
	core.TimeTrack()

	key := C.GoString(keyC)
	core.Start("called with dtH: %d, key: %s", dtH, key)
	dt, err := replicas.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	rows_, err := dt.Query(key, args)
	if err != nil {
		core.LogError("cannot execute query %s with args %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(nil, rows.Add(&rows_), err)
}

// bao_replica_sync synchronizes the SQL tables with the vault data.
//
//export bao_replica_sync
func bao_replica_sync(dtH C.longlong) C.Result {
	core.TimeTrack()
	core.Start("")
	replica, err := replicas.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d", dtH, err)
		return cResult(nil, 0, err)
	}

	updates, err := replica.Sync()
	if err != nil {
		core.LogError("cannot synchronize tables in sql layer %d", dtH, err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(updates, 0, nil)
}

// bao_replica_fetch fetches the specified number of rows from the specified rows.
//
//export bao_replica_fetch
func bao_replica_fetch(dtH C.longlong, keyC, argsC *C.char, maxRows C.int) C.Result {
	core.TimeTrack()

	key := C.GoString(keyC)
	core.Start("key: %s", key)
	dt, err := replicas.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	rows_, err := dt.Fetch(key, args, int(maxRows))
	if err != nil {
		core.LogError("cannot execute query %s with args %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(rows_, 0, nil)
}

// bao_replica_fetchOne fetches a single row for the specified SQL like query and arguments.
//
//export bao_replica_fetchOne
func bao_replica_fetchOne(dtH C.longlong, keyC, argsC *C.char) C.Result {
	core.TimeTrack()
	key := C.GoString(keyC)
	core.Start("key: %s", key)

	dt, err := replicas.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	values, err := dt.FetchOne(key, args)
	if err != nil {
		core.LogError("cannot execute query %s with args %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(values, 0, nil)
}

// bao_replica_current returns the next row from the specified rows.
//
//export bao_replica_current
func bao_replica_current(rowsH C.longlong) C.Result {
	core.TimeTrack()

	core.Start("rowsH: %d", rowsH)
	r, err := rows.Get(int64(rowsH))
	if err != nil {
		core.LogError("cannot get rows with handle %d", rowsH, err)
		return cResult(nil, 0, err)
	}

	values, err := r.Current()
	if err != nil {
		core.LogError("cannot get current row from rows %d", rowsH, err)
		return cResult(nil, 0, err)
	}
	core.End("%d values", len(values))
	return cResult(values, 0, err)
}

// bao_rowsNext checks if there are more rows to read.
//
//export bao_replica_next
func bao_replica_next(rowsH C.longlong) C.Result {
	core.TimeTrack()

	core.Start("rowsH: %d", rowsH)
	r, err := rows.Get(int64(rowsH))
	if err != nil {
		core.LogError("cannot get rows with handle %d", rowsH, err)
		return cResult(nil, 0, err)
	}

	res := r.Next()
	core.End("successfully checked for next row handle %d, more rows %t: %v", rowsH, res, r)
	return cResult(res, 0, nil)
}

// bao_replica_closeRows closes the specified rows.
//
//export bao_replica_closeRows
func bao_replica_closeRows(rowsH C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with rowsH: %d", rowsH)
	r, err := rows.Get(int64(rowsH))
	if err != nil {
		core.LogError("cannot get rows with handle %d", rowsH, err)
		return cResult(nil, 0, err)
	}

	err = r.Close()
	if err != nil {
		core.LogError("cannot close rows with handle %d", rowsH, err)
		return cResult(nil, 0, err)
	}
	rows.Remove(int64(rowsH))
	core.End("successfully closed rows with handle %d", rowsH)
	return cResult(nil, 0, nil)
}

// bao_replica_cancel rolls back the changes since the last sync
//
//export bao_replica_cancel
func bao_replica_cancel(dtH C.longlong) C.Result {
	core.TimeTrack()

	core.Start("called with dtH: %d", dtH)
	dt, err := replicas.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d: %v", dtH, err)
		return cResult(nil, 0, err)
	}

	err = dt.Cancel()
	if err != nil {
		core.LogError("cannot cancel sql layer %d", dtH, err)
	}
	core.End("successfully cancelled sql layer %d", dtH)
	return cResult(nil, 0, err)
}

// bao_mailbox_send sends the specified message using the specified dir as container
//
//export bao_mailbox_send
func bao_mailbox_send(sH C.longlong, dir, message *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dir: %s, message: %s", sH, C.GoString(dir), C.GoString(message))
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d", sH, err)
		return cResult(nil, 0, err)
	}

	var m mailbox.Message
	err = cInput(err, message, &m)
	if err != nil {
		core.LogError("cannot unmarshal message %s", C.GoString(message), err)
		return cResult(nil, 0, err)
	}

	err = mailbox.Send(s, C.GoString(dir), m)
	if err != nil {
		core.LogError("cannot send message %s", C.GoString(message), err)
		return cResult(nil, 0, err)
	}
	core.End("successfully sent message %s", C.GoString(message))
	return cResult(nil, 0, err)
}

// bao_mailbox_receive receives messages from the specified dir since the specified time and from the specified id.
//
//export bao_mailbox_receive
func bao_mailbox_receive(sH C.longlong, dir *C.char, since, fromId C.longlong) C.Result {
	core.TimeTrack()

	core.Start("called with sH: %d, dir: %s, since: %d, fromId: %d", sH, C.GoString(dir), since, fromId)
	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	msgs, err := mailbox.Receive(s, C.GoString(dir), time.UnixMilli(int64(since)), int64(fromId))
	if err != nil {
		core.LogError("cannot receive messages from vault %d", sH, err)
		return cResult(nil, 0, err)
	}
	core.End("successfully received messages from vault %d: %v", sH, msgs)
	return cResult(msgs, 0, err)
}

// bao_mailbox_download downloads the specified attachment for the specified message to the specified destination.
//
//export bao_mailbox_download
func bao_mailbox_download(sH C.longlong, dir, message *C.char, attachment C.int, dest *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dir: %s, message: %s, attachment: %d, dest: %s", sH, C.GoString(dir), C.GoString(message), attachment, C.GoString(dest))

	s, err := vaults.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get vault with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	var m mailbox.Message
	err = cInput(err, message, &m)
	if err != nil {
		core.LogError("cannot unmarshal message %s", C.GoString(message), err)
		return cResult(nil, 0, err)
	}

	err = mailbox.Download(s, C.GoString(dir), m, int(attachment), C.GoString(dest))
	if err != nil {
		core.LogError("cannot download attachment %d from message %s to %s", attachment, C.GoString(message), C.GoString(dest), err)
		return cResult(nil, 0, err)
	}
	core.End("successfully downloaded attachment %d from message %s to %s", attachment, C.GoString(message), C.GoString(dest))
	return cResult(nil, 0, err)
}

type Snapshot struct {
	DBs        *core.Registry[*sqlx.DB]
	vaults     *core.Registry[*vault.Vault]
	BaoQLayers *core.Registry[*replica.Replica]
}

// bao_snapshot creates a snapshot of all vaultes and baoqlayers.
//
//export bao_snapshot
func bao_snapshot() C.Result {
	core.TimeTrack()
	core.Start("creating snapshot of vaultes and sql layers")

	snapshot := Snapshot{
		DBs:        &dbs,
		vaults:     &vaults,
		BaoQLayers: &replicas,
	}

	data, err := yaml.Marshal(&snapshot)
	if err != nil {
		core.LogError("cannot marshal snapshot: %v", err)
		return cResult(nil, 0, err)
	}

	core.End("successfully created snapshot")
	return cResult(string(data), 0, nil)
}
