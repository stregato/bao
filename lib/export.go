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
	"encoding/base64"
	"encoding/json"
	"os"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/mailbox"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/bao_ql"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/bao"
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
		return core.Errorw("failed to unmarshal input - %v: %s", err, data)
	}
	return nil
}

var (
	dbs       core.Registry[*sqlx.DB]
	stashes   core.Registry[*bao.Bao]
	sqlLayers core.Registry[*bao_ql.BaoQL]
	rows      core.Registry[*sqlx.RowsX]
)

// bao_setLogLevel sets the log level for the stash library. Possible values are: trace, debug, info, warn, error, fatal, panic.
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

// bao_setHttpLog sets the HTTP endpoint for log messages.
//
//export bao_setHttpLog
func bao_setHttpLog(url *C.char) C.Result {
	core.Start("url %s", C.GoString(url))
	core.SetHttpLog(C.GoString(url))
	core.End("url %s", C.GoString(url))
	return cResult(nil, 0, nil)
}

// bao_getRecentLog retrieves the most recent log messages up to the specified number.
//
//export bao_getRecentLog
func bao_getRecentLog(n C.int) C.Result {
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

// bao_newPrivateID creates a new identity with the specified nick. An identity is a key pair used for encryption and signing, and a nick name for human readable identification.
// An identity is made of two fields ID and Private. ID is a concatenation of the nick name and the public key of the key pair. Private is the private key of the key pair.
//
//export bao_newPrivateID
func bao_newPrivateID() C.Result {
	core.Start("")
	core.TimeTrack()
	identity, err := security.NewPrivateID()
	if err != nil {
		core.LogError("cannot generate private ID: %v", err)
		core.End("failed to generate private ID")
		return cResult(nil, 0, err)
	}
	core.End("generated new private ID")
	return cResult(identity, 0, nil)
}

// bao_publicID returns the public ID of the specified identity.
//
//export bao_publicID
func bao_publicID(privateID *C.char) C.Result {
	privID := C.GoString(privateID)
	core.Start("private ID len %d", len(privID))
	core.TimeTrack()
	id, err := security.PrivateID(privID).PublicID()
	if err != nil {
		core.LogError("cannot derive public ID from provided private ID: %v", err)
		core.End("failed to derive public ID")
		return cResult(nil, 0, err)
	}
	core.End("public ID derived")
	return cResult(id, 0, nil)
}

// bao_ecEncrypt encrypts the specified plaintext using the provided identity.
//
//export bao_ecEncrypt
func bao_ecEncrypt(id *C.char, plainData C.Data) C.Result {
	idStr := C.GoString(id)
	core.Start("id len %d", len(idStr))
	core.TimeTrack()

	data := C.GoBytes(plainData.ptr, C.int(plainData.len))

	cipherData, err := security.EcEncrypt(security.PublicID(idStr), data)
	if err != nil {
		core.LogError("cannot encrypt plaintext with provided public ID: %v", err)
		core.End("failed for id len %d", len(idStr))
		return cResult(nil, 0, err)
	}
	core.End("id len %d", len(idStr))
	return cResult(cipherData, 0, nil)
}

// bao_ecDecrypt decrypts the specified ciphertext using the provided identity.
//
//export bao_ecDecrypt
func bao_ecDecrypt(id *C.char, cipherData C.Data) C.Result {
	idStr := C.GoString(id)
	core.Start("id len %d", len(idStr))
	core.TimeTrack()
	data := C.GoBytes(cipherData.ptr, C.int(cipherData.len))
	plainData, err := security.EcDecrypt(security.PrivateID(idStr), data)
	if err != nil {
		core.LogError("cannot decrypt ciphertext with provided private ID: %v", err)
		core.End("failed for id len %d", len(idStr))
		return cResult(nil, 0, err)
	}
	core.End("id len %d", len(idStr))
	return cResult(plainData, 0, nil)
}

// bao_aesEncrypt encrypts the specified plaintext using the provided key/nonce.
//
//export bao_aesEncrypt
func bao_aesEncrypt(key *C.char, nonceData, plainData C.Data) C.Result {
	keyStr := C.GoString(key)
	core.Start("key len %d", len(keyStr))
	core.TimeTrack()
	nonce := C.GoBytes(nonceData.ptr, C.int(nonceData.len))
	data := C.GoBytes(plainData.ptr, C.int(plainData.len))
	cipherText, err := security.AESEncrypt([]byte(keyStr), nonce, data)
	if err != nil {
		core.LogError("cannot encrypt plaintext with provided key (len %d) and nonce (%d bytes): %v", len(keyStr), len(nonce), err)
		core.End("failed for key len %d", len(keyStr))
		return cResult(nil, 0, err)
	}
	core.End("key len %d", len(keyStr))
	return cResult(cipherText, 0, nil)
}

// bao_aesDecrypt decrypts the specified ciphertext using the provided key/nonce.
//
//export bao_aesDecrypt
func bao_aesDecrypt(key *C.char, nonceData, cipherData C.Data) C.Result {
	keyStr := C.GoString(key)
	core.Start("key len %d", len(keyStr))
	core.TimeTrack()
	nonce := C.GoBytes(nonceData.ptr, C.int(nonceData.len))
	data := C.GoBytes(cipherData.ptr, C.int(cipherData.len))
	plainText, err := security.AESDecrypt([]byte(keyStr), nonce, data)
	if err != nil {
		core.LogError("cannot decrypt ciphertext with provided key (len %d) and nonce (%d bytes): %v", len(keyStr), len(nonce), err)
		core.End("failed for key len %d", len(keyStr))
		return cResult(nil, 0, err)
	}
	core.End("key len %d", len(keyStr))
	return cResult(plainText, 0, nil)
}

// bao_decodeID decodes the specified identity into the crypt key and the sign key.
//
//export bao_decodeID
func bao_decodeID(id *C.char) C.Result {
	idStr := C.GoString(id)
	core.Start("id len %d", len(idStr))
	core.TimeTrack()
	cryptKey, signKey, err := security.DecodeID(idStr)
	if err != nil {
		core.LogError("cannot decode ID: %v", err)
		core.End("failed to decode ID")
		return cResult(nil, 0, err)
	}

	cryptKey64 := base64.URLEncoding.EncodeToString(cryptKey)
	signKey64 := base64.URLEncoding.EncodeToString(signKey)

	core.End("decoded ID into key components")
	return cResult(map[string]string{"cryptKey": cryptKey64, "signKey": signKey64}, 0, err)
}

// bao_openDB opens a new database connection to the specified URL.Bao library requires a database connection to store safe and file system data. The function returns a handle to the database connection.
//
//export bao_openDB
func bao_openDB(driverName, dataSourceName, ddl *C.char) C.Result {
	core.Start("driver %s", C.GoString(driverName))
	var db *sqlx.DB
	var err error

	core.TimeTrack()

	db, err = sqlx.Open(C.GoString(driverName), C.GoString(dataSourceName), C.GoString(ddl))
	if err != nil {
		core.LogError("cannot open db with url %s: %v", C.GoString(dataSourceName), err)
		return cResult(nil, 0, err)
	}

	core.End("url %s", C.GoString(dataSourceName))
	return cResult(db, dbs.Add(db), err)
}

// bao_closeDB closes the specified database connection.
//
//export bao_closeDB
func bao_closeDB(dbH C.longlong) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}
	d.Close()
	dbs.Remove(int64(dbH))
	core.End("handle %d", dbH)
	return cResult(nil, 0, nil)
}

// bao_dbQuery executes the specified query on the database connection identified by the given handle.
//
//export bao_dbQuery
func bao_dbQuery(dbH C.longlong, queryC *C.char, argsC *C.char) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}

	query := C.GoString(queryC)

	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s: %v", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}

	res, err := d.Query(query, args)
	core.End("query %s", query)
	return cResult(nil, rows.Add(&res), err)
}

// bao_dbExec executes the specified query on the database connection identified by the given handle and returns a single row.
//
//export bao_dbExec
func bao_dbExec(dbH C.longlong, queryC *C.char, argsC *C.char) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}
	query := C.GoString(queryC)
	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s: %v", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}

	_, err = d.Exec(query, args)
	core.End("query %s", query)
	return cResult(nil, 0, err)
}

// bao_dbFetch fetches rows from the specified query on the database connection identified by the given handle.
//
//export bao_dbFetch
func bao_dbFetch(dbH C.longlong, queryC *C.char, argsC *C.char, maxRows C.int) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}
	query := C.GoString(queryC)
	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s: %v", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}

	rows, err := d.Fetch(query, args, int(maxRows))
	if err != nil {
		core.LogError("cannot fetch rows for query %s: %v", query, err)
		return cResult(nil, 0, err)
	}
	core.End("query %s", query)
	return cResult(rows, 0, nil)
}

// bao_dbFetchOne fetches a single row from the specified query on the database connection identified by the given handle.
//
//export bao_dbFetchOne
func bao_dbFetchOne(dbH C.longlong, queryC *C.char, argsC *C.char) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}
	query := C.GoString(queryC)
	var args map[string]any
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot unmarshal args %s: %v", C.GoString(argsC), err)
		return cResult(nil, 0, err)
	}
	row, err := d.FetchOne(query, args)
	if err != nil {
		core.LogError("cannot fetch one row for query %s: %v", query, err)
		return cResult(nil, 0, err)
	}
	core.End("query %s", query)
	return cResult(row, 0, nil)
}

// bao_create creates a new stash with the specified identity, URL and configuration. A stash is a secure storage for keys and files. The function returns a handle to the bao.
//
//export bao_create
func bao_create(dbH C.longlong, idC *C.char, storeC *C.char, settingsC *C.char) C.Result {
	core.Start("db handle %d", dbH)
	core.TimeTrack()
	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}

	var settings bao.Config
	err = cInput(err, settingsC, &settings)
	if err != nil {
		core.LogError("cannot unmarshal settings %s: %v", C.GoString(settingsC), err)
		return cResult(nil, 0, err)
	}

	id := C.GoString(idC)
	store := C.GoString(storeC)
	s, err := bao.Create(d, security.PrivateID(id), store, settings)
	if err != nil {
		core.LogError("cannot create stash for store %s: %v", store, err)
		return cResult(nil, 0, err)
	}

	core.End("created stash for store %s", store)
	return cResult(s, stashes.Add(s), err)
}

// bao_open opens an existing stash with the specified identity, author and URL. The function returns a handle to the bao.
//
//export bao_open
func bao_open(dbH C.longlong, idC *C.char, urlC *C.char, authorC *C.char) C.Result {
	core.Start("handle %d", dbH)
	core.TimeTrack()

	d, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}

	id := C.GoString(idC)
	url := C.GoString(urlC)
	author := C.GoString(authorC)

	s, err := bao.Open(d, security.PrivateID(id), url, security.PublicID(author))
	if err != nil {
		core.LogError("cannot open stash for store %s: %v", url, err)
		return cResult(nil, 0, err)
	}

	core.End("stash open for store %s", url)
	return cResult(s, stashes.Add(s), nil)
}

// bao_close closes the specified safe.
//
//export bao_close
func bao_close(sH C.longlong) C.Result {
	core.Start("handle %d", sH)
	core.TimeTrack()
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	s.Close()
	stashes.Remove(int64(sH))
	core.End("handle %d", sH)
	return cResult(nil, 0, nil)
}

// bao_syncAccess sets and optionally flushes access rights for the specified bao.
//
//export bao_syncAccess
func bao_syncAccess(sH C.longlong, optionsC C.int, changesC *C.char) C.Result {
	core.Start("handle %d", sH)
	core.TimeTrack()
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	var changes []bao.AccessChange
	changesJSON := C.GoString(changesC)
	if err := json.Unmarshal([]byte(changesJSON), &changes); err != nil {
		core.LogError("cannot unmarshal access change payload: %v", err)
		return cResult(nil, 0, err)
	}

	err = s.SyncAccess(bao.IOOption(optionsC), changes...)
	if err != nil {
		core.LogError("cannot synchronize access changes: %v", err)
		return cResult(nil, 0, err)
	}
	core.End("handled %d access changes", len(changes))
	return cResult(nil, 0, nil)
}

// bao_getAccess returns the users and access rights for the specified group.
//
//export bao_getAccess
func bao_getAccess(sH C.longlong, group *C.char) C.Result {
	core.Start("handle %d", sH)
	core.TimeTrack()
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	a, err := s.GetUsers(bao.Group(C.GoString(group)))
	if err != nil {
		core.LogError("cannot get access for stash %s: %v", C.GoString(group), err)
		return cResult(nil, 0, err)
	}
	core.End("group %s", C.GoString(group))
	return cResult(a, 0, nil)
}

// bao_getGroups returns the groups for the specified user.
//
//export bao_getGroups
func bao_getGroups(sH C.longlong, userC *C.char) C.Result {
	userID := C.GoString(userC)
	core.Start("handle %d user len %d", sH, len(userID))
	core.TimeTrack()
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	groups, err := s.GetGroups(security.PublicID(userID))
	if err != nil {
		core.LogError("cannot get groups for provided user in stash %d: %v", sH, err)
		return cResult(nil, 0, err)
	}
	core.End("user len %d", len(userID))
	return cResult(groups, 0, nil)
}

// bao_syncFS synchronizes the file system for the specified groups in the bao.
//
// bao_sync synchronizes the file system for the specified groups in the bao.

//export bao_sync
func bao_sync(sH C.longlong, groupsC *C.char) C.Result {
	var groups []bao.Group

	core.TimeTrack()
	core.Start("handle %d", sH)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	if groupsC != nil {
		err = cInput(err, groupsC, &groups)
		if err != nil {
			core.LogError("cannot unmarshal groups %s: %v", C.GoString(groupsC), err)
			return cResult(nil, 0, err)
		}
	}

	files, err := s.Sync(groups...)
	if err != nil {
		core.LogError("cannot synchronize groups %v in stash %d: %v", groups, sH, err)
		return cResult(nil, 0, err)
	}

	core.End("bao_sync successful for stash %d with groups: %v", sH, groups)
	return cResult(files, 0, nil)
}

// bao_listGroups returns all the groups in the stack
//
//export bao_listGroups
func bao_listGroups(sH C.longlong) C.Result {
	core.TimeTrack()
	core.Start("handle %d", sH)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	groups, err := s.ListGroups()
	if err != nil {
		core.LogError("cannot list groups for stash %d: %v", sH, err)
		return cResult(nil, 0, err)
	}
	core.End("stash with handle %d groups: %v", sH, groups)
	return cResult(groups, 0, nil)
}

// bao_waitFiles completes the read and write operations for the specified files in the bao.
//
//export bao_waitFiles
func bao_waitFiles(sH C.longlong, fileIdsC *C.char) C.Result {
	var fileIds []bao.FileId

	core.TimeTrack()
	core.Start("called with sH: %d", sH)
	s, err := stashes.Get(int64(sH))
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

	err = s.WaitFiles(fileIds...)
	if err != nil {
		logrus.Errorf("cannot synchronize stash %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	core.End("bao_sync successful for stash %d with file IDs: %v", sH, fileIds)
	return cResult(nil, 0, nil)
}

// bao_setAttribute sets an attribute for the current user
//
//export bao_setAttribute
func bao_setAttribute(sH C.longlong, options C.int, nameC, valueC *C.char) C.Result {
	core.TimeTrack()
	core.Start("handle %d", sH)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	name := C.GoString(nameC)
	value := C.GoString(valueC)

	err = s.SetAttribute(bao.IOOption(options), name, value)
	if err != nil {
		core.LogError("cannot set attribute %s to %s in stash %d: %v", name, value, sH, err)
		return cResult(nil, 0, err)
	}
	core.End("set attribute %s to %s in stash %d", name, value, sH)
	return cResult(nil, 0, nil)
}

// bao_getAttribute gets an attribute for the specified user
//
//export bao_getAttribute
func bao_getAttribute(sH C.longlong, nameC, authorC *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s, author: %s", sH, C.GoString(nameC), C.GoString(authorC))
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	name := C.GoString(nameC)
	author := C.GoString(authorC)
	authorLen := len(author)

	value, err := s.GetAttribute(name, security.PublicID(author))
	if err != nil {
		core.LogError("cannot get attribute '%s' for provided user (len %d) in stash %d: %v", name, authorLen, sH, err)
		return cResult(nil, 0, err)
	}
	core.End("got attribute '%s' for user len %d in stash %d: %s", name, authorLen, sH, value)
	return cResult(value, 0, nil)
}

// bao_getAttributes gets all attributes for the specified user
//
//export bao_getAttributes
func bao_getAttributes(sH C.longlong, authorC *C.char) C.Result {
	core.TimeTrack()
	core.Start("handle %d", sH)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	author := C.GoString(authorC)
	authorLen := len(author)

	attrs, err := s.GetAttributes(security.PublicID(author))
	if err != nil {
		core.LogError("cannot get attributes for provided user (len %d) in stash %d: %v", authorLen, sH, err)
		return cResult(nil, 0, err)
	}
	core.End("got attributes for user len %d in stash %d: %v", authorLen, sH, attrs)
	return cResult(attrs, 0, nil)
}

// bao_readDir reads the specified directory from the bao.
//
//export bao_readDir
func bao_readDir(sH C.longlong, dir *C.char, after, fromId C.longlong, limit C.int) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dir: %s, after: %d, fromId: %d, limit: %d", sH, C.GoString(dir), after, fromId, limit)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	list, err := s.ReadDir(C.GoString(dir), time.Unix(int64(after), 0), int64(fromId), int(limit))
	core.End("bao_readDir returning: %v", list)

	return cResult(list, 0, err)
}

// bao_stat returns the file information for the specified file in the bao.
//
//export bao_stat
func bao_stat(sH C.longlong, name *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s", sH, C.GoString(name))
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	info, err := s.Stat(C.GoString(name))
	if os.IsNotExist(err) {
		return cResult(nil, 0, err)
	}
	if err != nil {
		core.LogError("cannot get file info for %s in stash %d: %v", C.GoString(name), sH, err)
		return cResult(nil, 0, err)
	}
	core.End("successful statistic for file %s in stash %d: %v", C.GoString(name), sH, info)
	return cResult(info, 0, err)
}

// bao_getGroup returns the group name of the specified file.
//
//export bao_getGroup
func bao_getGroup(sH C.longlong, name *C.char) C.Result {
	core.TimeTrack()
	core.Start("bao_getGroup called with sH: %d, name: %s", sH, C.GoString(name))
	s, err := stashes.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	group, err := s.GetGroup(C.GoString(name))
	core.End("successful group retrieval for file %s in stash %d: %v", C.GoString(name), sH, group)
	return cResult(group, 0, err)
}

// bao_getAuthor returns the author of the specified file.
//
// bao_waitFiles waits for pending I/O for the specified files in the bao.
//
//export bao_getAuthor
func bao_getAuthor(sH C.longlong, name *C.char) C.Result {
	core.TimeTrack()
	core.Start("bao_getAuthor called with sH: %d, name: %s", sH, C.GoString(name))
	s, err := stashes.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	author, err := s.GetAuthor(C.GoString(name))
	core.End("successful author retrieval for file %s in stash %d: %v", C.GoString(name), sH, author)
	return cResult(author, 0, err)
}

// bao_read reads the specified file from the bao.
//
//export bao_read
func bao_read(sH C.longlong, name, destC *C.char, options C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s, dest: %s, options: %d", sH, C.GoString(name), C.GoString(destC), options)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		return cResult(nil, 0, err)
	}

	dest := C.GoString(destC)
	file, err := s.Read(C.GoString(name), dest, bao.IOOption(options), nil)
	if err != nil {
		core.LogError("cannot read file %s from stash %d: %v", C.GoString(name), sH, err)
		return cResult(nil, 0, err)
	}

	core.End("successfully read file %s from stash %d to %s", C.GoString(name), sH, C.GoString(destC))
	return cResult(file, 0, err)
}

// bao_write writes the specified file to the bao.
//
//export bao_write
func bao_write(sH C.longlong, destC, sourceC, groupC *C.char, attrsC C.Data, options C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dest: %s, source: %s, group: %s, options: %d", sH, C.GoString(destC), C.GoString(sourceC), C.GoString(groupC), options)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	dest := C.GoString(destC)
	source := C.GoString(sourceC)
	group := C.GoString(groupC)
	attrs := C.GoBytes(unsafe.Pointer(attrsC.ptr), C.int(attrsC.len))

	file, err := s.Write(dest, source, bao.Group(group), attrs, bao.IOOption(options), nil)
	if err != nil {
		core.LogError("cannot write file %s to stash %d, src %s: %v", dest, sH, source, err)
		return cResult(nil, 0, err)
	}

	core.End("successfully wrote file %s to stash %d, src %s", dest, sH, source)
	return cResult(file, 0, nil)
}

// bao_delete deletes the specified file from the bao.
//
//export bao_delete
func bao_delete(sH C.longlong, nameC *C.char, options int) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, name: %s", sH, C.GoString(nameC))
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	name := C.GoString(nameC)
	err = s.Delete(name, bao.IOOption(options))
	if err != nil {
		core.LogError("cannot delete file %s from stash %d: %v", name, sH, err)
		return cResult(nil, 0, err)
	}

	core.End("successfully deleted file %s from stash %d", name, sH)
	return cResult(nil, 0, nil)
}

// stassh_allocatedSize returns the allocated size of the specified bao.
//
//export bao_allocatedSize
func bao_allocatedSize(sH C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d", sH)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	size := s.AllocatedSize()
	core.End("allocated size for stash %d: %d", sH, size)
	return cResult(size, 0, nil)
}

// baoql_layer returns a SQL like layer for the specified bao. The layer is used to execute SQL like commands on the stash data.
//
//export baoql_layer
func baoql_layer(sH C.longlong, groupC *C.char, dbH C.int) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, group: %s, dbH: %d", sH, C.GoString(groupC), dbH)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	group := bao.Group(C.GoString(groupC))

	db, err := dbs.Get(int64(dbH))
	if err != nil {
		core.LogError("cannot get db with handle %d: %v", dbH, err)
		return cResult(nil, 0, err)
	}

	dt, err := bao_ql.BaoQLayer(s, group, db)
	if err != nil {
		core.LogError("cannot create sql layer for stash %d: %v", sH, err)
		return cResult(nil, 0, err)
	}
	core.End("sql layer for stash %d created", sH)
	return cResult(dt, sqlLayers.Add(dt), err)
}

// baoql_exec executes the specified SQL like command on the specified data table.
//
//export baoql_exec
func baoql_exec(dtH C.longlong, keyC, argsC *C.char) C.Result {
	core.TimeTrack()

	key := C.GoString(keyC)
	core.Start("dtH: %d, key: %s", dtH, key)

	dt, err := sqlLayers.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s: %v", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v: %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	_, err = dt.Exec(C.GoString(keyC), args)
	if err != nil {
		core.LogError("cannot execute query %s with args %v: %v", key, map[string]any(args), err)
	}
	core.End("")
	return cResult(nil, 0, err)
}

// baoql_query executes the specified SQL like query on the specified data table.
//
//export baoql_query
func baoql_query(dtH C.longlong, keyC, argsC *C.char) C.Result {
	core.TimeTrack()

	key := C.GoString(keyC)
	core.Start("called with dtH: %d, key: %s", dtH, key)
	dt, err := sqlLayers.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s: %v", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v: %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	rows_, err := dt.Query(key, args)
	if err != nil {
		core.LogError("cannot execute query %s with args %v: %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(nil, rows.Add(&rows_), err)
}

// baoql_sync_tables synchronizes the SQL tables with the stash data.
//
//export baoql_sync_tables
func baoql_sync_tables(dtH C.longlong) C.Result {
	core.TimeTrack()
	core.Start("")
	dt, err := sqlLayers.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d: %v", dtH, err)
		return cResult(nil, 0, err)
	}

	updates, err := dt.SyncTables()
	if err != nil {
		core.LogError("cannot synchronize tables in sql layer %d: %v", dtH, err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(updates, 0, nil)
}

// baoql_fetch fetches the specified number of rows from the specified rows.
//
//export baoql_fetch
func baoql_fetch(dtH C.longlong, keyC, argsC *C.char, maxRows C.int) C.Result {
	core.TimeTrack()

	key := C.GoString(keyC)
	core.Start("key: %s", key)
	dt, err := sqlLayers.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s: %v", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v: %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	rows_, err := dt.Fetch(key, args, int(maxRows))
	if err != nil {
		core.LogError("cannot execute query %s with args %v: %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(rows_, 0, nil)
}

// baoql_fetchOne fetches a single row for the specified SQL like query and arguments.
//
//export baoql_fetchOne
func baoql_fetchOne(dtH C.longlong, keyC, argsC *C.char) C.Result {
	core.TimeTrack()
	key := C.GoString(keyC)
	core.Start("key: %s", key)

	dt, err := sqlLayers.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d for query %s: %v", dtH, key, err)
		return cResult(nil, 0, err)
	}

	var args sqlx.Args
	err = cInput(err, argsC, &args)
	if err != nil {
		core.LogError("cannot convert input for query %s with args %v: %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}

	values, err := dt.FetchOne(key, args)
	if err != nil {
		core.LogError("cannot execute query %s with args %v: %v", key, map[string]any(args), err)
		return cResult(nil, 0, err)
	}
	core.End("")
	return cResult(values, 0, nil)
}

// baoql_current returns the next row from the specified rows.
//
//export baoql_current
func baoql_current(rowsH C.longlong) C.Result {
	core.TimeTrack()

	core.Start("rowsH: %d", rowsH)
	r, err := rows.Get(int64(rowsH))
	if err != nil {
		core.LogError("cannot get rows with handle %d: %v", rowsH, err)
		return cResult(nil, 0, err)
	}

	values, err := r.Current()
	if err != nil {
		core.LogError("cannot get current row from rows %d: %v", rowsH, err)
		return cResult(nil, 0, err)
	}
	core.End("%d values", len(values))
	return cResult(values, 0, err)
}

// bao_rowsNext checks if there are more rows to read.
//
//export baoql_next
func baoql_next(rowsH C.longlong) C.Result {
	core.TimeTrack()

	core.Start("rowsH: %d", rowsH)
	r, err := rows.Get(int64(rowsH))
	if err != nil {
		core.LogError("cannot get rows with handle %d: %v", rowsH, err)
		return cResult(nil, 0, err)
	}

	res := r.Next()
	core.End("successfully checked for next row handle %d, more rows %t: %v", rowsH, res, r)
	return cResult(res, 0, nil)
}

// baoql_closeRows closes the specified rows.
//
//export baoql_closeRows
func baoql_closeRows(rowsH C.longlong) C.Result {
	core.TimeTrack()
	core.Start("called with rowsH: %d", rowsH)
	r, err := rows.Get(int64(rowsH))
	if err != nil {
		core.LogError("cannot get rows with handle %d: %v", rowsH, err)
		return cResult(nil, 0, err)
	}

	err = r.Close()
	if err != nil {
		core.LogError("cannot close rows with handle %d: %v", rowsH, err)
		return cResult(nil, 0, err)
	}
	rows.Remove(int64(rowsH))
	core.End("successfully closed rows with handle %d", rowsH)
	return cResult(nil, 0, nil)
}

// baoql_cancel rolls back the changes since the last sync
//
//export baoql_cancel
func baoql_cancel(dtH C.longlong) C.Result {
	core.TimeTrack()

	core.Start("called with dtH: %d", dtH)
	dt, err := sqlLayers.Get(int64(dtH))
	if err != nil {
		core.LogError("cannot get sql layer %d: %v", dtH, err)
		return cResult(nil, 0, err)
	}

	err = dt.Cancel()
	if err != nil {
		core.LogError("cannot cancel sql layer %d: %v", dtH, err)
	}
	core.End("successfully cancelled sql layer %d", dtH)
	return cResult(nil, 0, err)
}

// mailbox_send sends the specified message using the specified dir as container
//
//export mailbox_send
func mailbox_send(sH C.longlong, dir, group, message *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dir: %s, group: %s, message: %s", sH, C.GoString(dir), C.GoString(group), C.GoString(message))
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	var m mailbox.Message
	err = cInput(err, message, &m)
	if err != nil {
		core.LogError("cannot unmarshal message %s: %v", C.GoString(message), err)
		return cResult(nil, 0, err)
	}

	err = mailbox.Send(s, C.GoString(dir), bao.Group(C.GoString(group)), m)
	if err != nil {
		core.LogError("cannot send message %s to group %s: %v", C.GoString(message), C.GoString(group), err)
		return cResult(nil, 0, err)
	}
	core.End("successfully sent message %s to group %s", C.GoString(message), C.GoString(group))
	return cResult(nil, 0, err)
}

// mailbox_receive receives messages from the specified dir since the specified time and from the specified id.
//
//export mailbox_receive
func mailbox_receive(sH C.longlong, dir *C.char, since, fromId C.longlong) C.Result {
	core.TimeTrack()

	core.Start("called with sH: %d, dir: %s, since: %d, fromId: %d", sH, C.GoString(dir), since, fromId)
	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	msgs, err := mailbox.Receive(s, C.GoString(dir), time.UnixMilli(int64(since)), int64(fromId))
	if err != nil {
		core.LogError("cannot receive messages from stash %d: %v", sH, err)
		return cResult(nil, 0, err)
	}
	core.End("successfully received messages from stash %d: %v", sH, msgs)
	return cResult(msgs, 0, err)
}

// mailbox_download downloads the specified attachment for the specified message to the specified destination.
//
//export mailbox_download
func mailbox_download(sH C.longlong, dir, message *C.char, attachment C.int, dest *C.char) C.Result {
	core.TimeTrack()
	core.Start("called with sH: %d, dir: %s, message: %s, attachment: %d, dest: %s", sH, C.GoString(dir), C.GoString(message), attachment, C.GoString(dest))

	s, err := stashes.Get(int64(sH))
	if err != nil {
		core.LogError("cannot get stash with handle %d: %v", sH, err)
		return cResult(nil, 0, err)
	}

	var m mailbox.Message
	err = cInput(err, message, &m)
	if err != nil {
		core.LogError("cannot unmarshal message %s: %v", C.GoString(message), err)
		return cResult(nil, 0, err)
	}

	err = mailbox.Download(s, C.GoString(dir), m, int(attachment), C.GoString(dest))
	if err != nil {
		core.LogError("cannot download attachment %d from message %s to %s: %v", attachment, C.GoString(message), C.GoString(dest), err)
		return cResult(nil, 0, err)
	}
	core.End("successfully downloaded attachment %d from message %s to %s", attachment, C.GoString(message), C.GoString(dest))
	return cResult(nil, 0, err)
}

type Snapshot struct {
	DBs       *core.Registry[*sqlx.DB]
	Stashes   *core.Registry[*bao.Bao]
	BaoQLayers *core.Registry[*bao_ql.BaoQL]
}

// bao_snapshot creates a snapshot of all stashes and baoqlayers.
//
//export bao_snapshot
func bao_snapshot() C.Result {
	core.TimeTrack()
	core.Start("creating snapshot of stashes and sql layers")

	snapshot := Snapshot{
		DBs:       &dbs,
		Stashes:   &stashes,
		BaoQLayers: &sqlLayers,
	}

	data, err := yaml.Marshal(&snapshot)
	if err != nil {
		core.LogError("cannot marshal snapshot: %v", err)
		return cResult(nil, 0, err)
	}

	core.End("successfully created snapshot")
	return cResult(string(data), 0, nil)
}
