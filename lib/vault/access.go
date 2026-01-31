package vault

import (
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

type AccessChange struct {
	UserId security.PublicID `json:"userId"`
	Access Access            `json:"access"`
}

// SyncAccess applies the provided access changes and optionally flushes them to the store.
func (v *Vault) SyncAccess(options IOOption, changes ...AccessChange) error {
	core.Start("syncing access changes %v with options %d", changes, options)
	if len(changes) == 0 {
		core.End("no access changes provided")
		return nil
	}

	nChanges := 0
	var err error
	cs, err := v.convertToChanges(changes)
	if err != nil {
		return core.Error(core.AuthError, "cannot convert access changes for domain %s", v.Realm, err)
	}
	for _, c := range cs {
		bc, err := marshalChange(c)
		if err != nil {
			return core.Error(core.GenericError, "cannot match block changes for domain %s", v.Realm, err)
		}
		err = v.stageBlockChange(bc)
		if err != nil {
			return core.Error(core.GenericError, "cannot stage block change for domain %s", v.Realm, err)
		}
		core.Info("staged %s in %s", c, v.ID)
	}
	nChanges += len(cs)

	switch {
	case options&AsyncOperation != 0:
		go v.syncBlockChain()
	case options&ScheduledOperation != 0:
		// do nothing, will be synced later
	default:
		if err := v.syncBlockChain(); err != nil {
			return core.Error(core.AuthError, "cannot synchronize blockchain for access changes", err)
		}
	}

	core.End("%d changes", nChanges)
	return nil
}

func (v *Vault) convertToChanges(changes []AccessChange) ([]Change, error) {
	core.Start("staging %d access changes", len(changes))

	current, err := v.GetAccesses()
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get existing access rights: %v", err)
	}
	needNewKey := len(current) == 0

	adminRight := v.UserPublicID == v.Author
	if !adminRight {
		if acc, ok := current[v.UserPublicID]; ok && acc&Admin > 0 {
			adminRight = true
		}
	}
	if !adminRight {
		return nil, core.Error(core.AuthError, "only the vault creator or an admin can change the access rights")
	}

	keysForScope, err := v.getKeysForScope()
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get keys: %v", err)
	}

	var delta []Change
	for _, change := range changes {
		currentAccess := current[change.UserId]
		if currentAccess == change.Access {
			continue
		}

		if currentAccess == 0 && len(keysForScope) > 0 {
			activeKeySet := ActiveKeySet{
				Id:   change.UserId,
				Keys: make(map[uint64][]byte),
			}
			for keyId, key := range keysForScope {
				encKey, encErr := security.EcEncrypt(change.UserId, key)
				if encErr != nil {
					return nil, core.Error(core.EncodeError, "cannot encrypt key for user %s: %v", change.UserId, encErr)
				}
				activeKeySet.Keys[keyId] = encKey
			}
			delta = append(delta, &activeKeySet)
		}

		delta = append(delta, &ChangeAccess{
			PublicID: change.UserId,
			Access:   change.Access,
		})
		needNewKey = needNewKey || change.Access == 0
		current[change.UserId] = change.Access
	}

	if needNewKey {
		keyId := core.SnowID() &^ (1 << 62)
		key := core.GenerateRandomBytes(32)
		if err := v.setKeyToDB(keyId, key); err != nil {
			return nil, err
		}

		recipients := make([]security.PublicID, 0, len(current))
		for id, access := range current {
			if access != 0 {
				recipients = append(recipients, id)
			}
		}

		if len(recipients) > 0 {
			addKey, err := v.createAddKey(recipients, keyId, key)
			if err != nil {
				return nil, core.Error(core.GenericError, "cannot create add key for domain %s", v.Realm, err)
			}
			delta = append(delta, &addKey)
			keysForScope = map[uint64]security.AESKey{keyId: key}
		}
	}

	core.Info("successfully created %d changes for domain %s", len(delta), v.Realm)
	core.End("")
	return delta, nil
}

func (v *Vault) createAddKey(ids []security.PublicID, keyId uint64, key security.AESKey) (AddKey, error) {
	core.Start("creating add key for keyId %d", keyId)
	// Create a AddKey object with the given keyId and key
	addKey := AddKey{
		KeyId:         keyId,
		EncryptedKeys: make(map[security.PublicID][]byte),
	}
	// Populate the EncodedKeys map with the new access rights
	for _, id := range ids {
		ekey, err := security.EcEncrypt(id, key)
		if err != nil {
			return AddKey{}, core.Error(core.EncodeError, "cannot encrypt key for user %s in domain %s", id, v.Realm, err)
		}
		addKey.EncryptedKeys[id] = ekey
	}
	core.End("created add key for domain %s, keyId %d", v.Realm, keyId)
	return addKey, nil
}

func (v *Vault) getUserByShortId(shortId uint64) (security.PublicID, error) {
	var id security.PublicID

	core.Start("getting user by short ID %d", shortId)
	err := v.DB.QueryRow("GET_USER_ID_BY_SHORT_ID", sqlx.Args{
		"vault":   v.ID,
		"shortId": shortId,
	}, &id)

	return id, err
}

// GetAccesses retrieves the access rights.
func (v *Vault) GetAccesses() (Accesses, error) {
	core.Start("group %s", v.Realm)
	var accesses Accesses = make(Accesses)

	rows, err := v.DB.Query("GET_ACCESSES", sqlx.Args{"vault": v.ID})
	if err == sqlx.ErrNoRows {
		core.End("no users")
		// no users found, return empty access
		return accesses, nil
	}
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get users for group %s", v.Realm, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id security.PublicID
		var access Access
		err = rows.Scan(&id, &access)
		if err != nil {
			return nil, core.Error(core.GenericError, "cannot scan user %s for group %s", id, v.Realm, err)
		}
		accesses[id] = access
	}
	core.End("%d users", len(accesses))
	return accesses, nil
}

// GetAccess returns the access for a user
func (v *Vault) GetAccess(publicID security.PublicID) (Access, error) {
	core.Start("user %s", publicID)
	var access Access

	err := v.DB.QueryRow("GET_ACCESS", sqlx.Args{
		"vault":  v.ID,
		"userId": publicID,
	}, &access)
	if err == sqlx.ErrNoRows {
		core.End("access 0")
		return 0, nil // No access
	}

	core.End("access %d", access)
	return access, err
}
