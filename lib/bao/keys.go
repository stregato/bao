package bao

import (
	"slices"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

const (
	NoKey uint64 = 0
	EcKey uint64 = 0x1
)

func (s *Bao) getLastKeyFromDB(group Group) (id uint64, key security.AESKey, err error) {
	core.Start("getting last key for group %s", group)
	if group == Public {
		return 0, nil, nil
	}

	err = s.DB.QueryRow("GET_LAST_KEY", sqlx.Args{"store": s.Id, "group": group}, &id, &key)
	if err != nil {
		return 0, nil, core.Errorw("cannot get last key for group %s", group, err)
	}
	core.End("successfully got last key for group %s: id=%d, key=%x", group, id, key)
	return id, key, nil
}

func (s *Bao) setKeyToDB(keyId uint64, group Group, key []byte) error {
	core.Start("setting key %d for group %s", keyId, group)

	_, err := s.DB.Exec("SET_KEY", sqlx.Args{"store": s.Id, "id": keyId, "key": key, "group": group, "tm": core.Now().Unix()})
	if err != nil {
		return core.Errorw("cannot set key %d for group %s", keyId, group, err)
	}

	// Update the local cache of groups if this is a new group
	if len(group) < 32 && !slices.Contains(s.groups, group) {
		s.groups = append(s.groups, group)
	}

	core.End("successfully set key %d for group %s", keyId, group)
	return err
}

func (s *Bao) getKey(id uint64) (key security.AESKey, err error) {
	core.Start("getting key %d", id)
	if id == NoKey {
		core.End("no key for public group")
		return nil, nil // No key for public group
	}
	err = s.DB.QueryRow("GET_KEY", sqlx.Args{"id": id}, &key)
	if err == sqlx.ErrNoRows {
		s.syncBlockChain() // Try to sync blockchain and get the key again
		err = s.DB.QueryRow("GET_KEY", sqlx.Args{"id": id}, &key)
	}
	if err != nil {
		if err == sqlx.ErrNoRows {
			return nil, core.Errorw(ErrNotAuthorized, id, err)
		}
		return nil, core.Errorw("cannot get key %d from DB", id, err)
	}
	core.End("successfully got key %d: %x", id, key)
	return key, nil
}

func (s *Bao) getKeysForScope(group Group) (map[uint64]security.AESKey, error) {
	core.Start("getting keys for group %s", group)
	rows, err := s.DB.Query("GET_KEYS_FOR_SCOPE", sqlx.Args{"store": s.Id, "group": group})
	if err != nil {
		return nil, core.Errorw("cannot get keys for group %s", group, err)
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
	core.End("successfully got %d keys for group %s", len(keys), group)
	return keys, nil
}

func (s *Bao) getGroupFromKey(id uint64) (group Group, err error) {
	core.Start("getting group for key %d", id)
	if id == NoKey {
		return Public, nil // No key for public group
	}

	err = s.DB.QueryRow("GET_SCOPE", sqlx.Args{"id": id}, &group)
	if err != nil {
		if err == sqlx.ErrNoRows {
			return Public, core.Errorw("no group found for id %d", id, err)
		}
		return Public, core.Errorw("cannot get group %d from DB", id, err)
	}
	core.End("successfully got group %d: %s", id, group)
	return group, nil
}
