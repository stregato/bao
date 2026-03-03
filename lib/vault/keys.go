package vault

import (
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

const (
	NoKey uint64 = 0
	EcKey uint64 = 0x1
)

func (v *Vault) getLastKeyFromDB() (id uint64, key security.AESKey, err error) {
	core.Start("getting last key for vault %s", v.ID)

	err = v.DB.QueryRow("GET_LAST_KEY", sqlx.Args{"vault": v.ID}, &id, &key)
	if err != nil {
		return 0, nil, core.Error(core.DbError, "cannot get last key for vault %s", v.ID, err)
	}
	core.End("successfully got last key for vault %s: id=%d, key=%x", v.ID, id, key)
	return id, key, nil
}

func (v *Vault) setKeyToDB(keyId uint64, key []byte) error {
	core.Start("setting key %d for vault %s", keyId, v.ID)

	_, err := v.DB.Exec("SET_KEY", sqlx.Args{"vault": v.ID, "id": keyId, "key": key, "tm": core.Now().Unix()})
	if err != nil {
		return core.Error(core.DbError, "cannot set key %d for vault %s", keyId, v.ID, err)
	}

	core.End("successfully set key %d for vault %s", keyId, v.ID)
	return err
}

func (v *Vault) getKey(id uint64) (key security.AESKey, err error) {
	core.Start("getting key %d", id)
	if id == NoKey {
		core.End("no key for public group")
		return nil, nil // No key for public group
	}
	err = v.DB.QueryRow("GET_KEY", sqlx.Args{"id": id}, &key)
	if err == sqlx.ErrNoRows {
		v.syncBlockChain(false) // Try to sync blockchain and get the key again
		err = v.DB.QueryRow("GET_KEY", sqlx.Args{"id": id}, &key)
	}
	if err != nil {
		if err == sqlx.ErrNoRows {
			return nil, core.Error(core.AccessDenied, "access denied for key %d", id, err)
		}
		return nil, core.Error(core.DbError, "cannot get key %d from DB", id, err)
	}
	core.End("successfully got key %d: %x", id, key)
	return key, nil
}

func (v *Vault) getKeysForScope() (map[uint64]security.AESKey, error) {
	core.Start("getting keys for vault %s", v.ID)
	rows, err := v.DB.Query("GET_KEYS", sqlx.Args{"vault": v.ID})
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get keys for vault %s", v.ID, err)
	}
	defer rows.Close()

	keys := make(map[uint64]security.AESKey)
	for rows.Next() {
		var id uint64
		var key security.AESKey
		err = rows.Scan(&id, &key)
		if err != nil {
			return nil, core.Error(core.DbError, "cannot scan key from DB", err)
		}
		keys[id] = key
	}
	core.End("successfully got %d keys for vault %s", len(keys), v.ID)
	return keys, nil
}
