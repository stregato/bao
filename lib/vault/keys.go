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
	core.Start("getting last key for group %s", v.Realm)
	if v.Realm == All {
		return 0, nil, nil
	}

	err = v.DB.QueryRow("GET_LAST_KEY", sqlx.Args{"vault": v.ID}, &id, &key)
	if err != nil {
		return 0, nil, core.Errorw("cannot get last key for group %s", v.Realm, err)
	}
	core.End("successfully got last key for group %s: id=%d, key=%x", v.Realm, id, key)
	return id, key, nil
}

func (v *Vault) setKeyToDB(keyId uint64, key []byte) error {
	core.Start("setting key %d for domain %s", keyId, v.Realm)

	_, err := v.DB.Exec("SET_KEY", sqlx.Args{"vault": v.ID, "id": keyId, "key": key, "tm": core.Now().Unix()})
	if err != nil {
		return core.Errorw("cannot set key %d for domain %s", keyId, v.Realm, err)
	}

	core.End("successfully set key %d for domain %s", keyId, v.Realm)
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
		v.syncBlockChain() // Try to sync blockchain and get the key again
		err = v.DB.QueryRow("GET_KEY", sqlx.Args{"id": id}, &key)
	}
	if err != nil {
		if err == sqlx.ErrNoRows {
			return nil, core.Errorw(ErrAccessDenied, id, err)
		}
		return nil, core.Errorw("cannot get key %d from DB", id, err)
	}
	core.End("successfully got key %d: %x", id, key)
	return key, nil
}

func (v *Vault) getKeysForScope() (map[uint64]security.AESKey, error) {
	core.Start("getting keys for domain %s", v.Realm)
	rows, err := v.DB.Query("GET_KEYS", sqlx.Args{"vault": v.ID})
	if err != nil {
		return nil, core.Errorw("cannot get keys for group %s", v.Realm, err)
	}
	defer rows.Close()

	keys := make(map[uint64]security.AESKey)
	for rows.Next() {
		var id uint64
		var key security.AESKey
		err = rows.Scan(&id, &key)
		if err != nil {
			return nil, core.Errorw("cannot scan key from DB", err)
		}
		keys[id] = key
	}
	core.End("successfully got %d keys for group %s", len(keys), v.Realm)
	return keys, nil
}

func (v *Vault) getGroupFromKey(id uint64) (group Realm, err error) {
	core.Start("getting group for key %d", id)
	if id == NoKey {
		return All, nil // No key for public group
	}

	err = v.DB.QueryRow("GET_SCOPE", sqlx.Args{"id": id}, &group)
	if err != nil {
		if err == sqlx.ErrNoRows {
			return All, core.Errorw("no group found for id %d", id, err)
		}
		return All, core.Errorw("cannot get group %d from DB", id, err)
	}
	core.End("successfully got group %d: %s", id, group)
	return group, nil
}
