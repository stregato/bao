package bao

import (
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

type AccessChange struct {
	Group  Group             `json:"group"`
	Access Access            `json:"access"`
	UserId security.PublicID `json:"userId"`
}

// SyncAccess applies the provided access changes and optionally flushes them to the store.
func (s *Bao) SyncAccess(options IOOption, accessChanges ...AccessChange) error {
	core.Start("syncing access changes %v with options %d", accessChanges, options)
	if len(accessChanges) == 0 {
		core.End("no access changes provided")
		return nil
	}

	if s.store == nil {
		store, err := storage.Open(s.StoreConfig)
		if err != nil {
			return core.Errorw("cannot open store with connection URL %s", s.StoreConfig, err)
		}
		s.store = store
	}

	grouped := make(map[Group][]AccessChange)
	order := make([]Group, 0)
	for _, accessChange := range accessChanges {
		if len(grouped[accessChange.Group]) == 0 {
			order = append(order, accessChange.Group)
		}

		grouped[accessChange.Group] = append(grouped[accessChange.Group], accessChange)
	}

	nChanges := 0
	for _, group := range order {
		var err error
		cs, err := s.convertToChanges(group, grouped[group])
		if err != nil {
			return core.Errorw("cannot convert access changes for group %s", group, err)
		}
		for _, c := range cs {
			bc, err := marshalChange(c)
			if err != nil {
				return core.Errorw("cannot match block changes for group %s", group, err)
			}
			s.stageBlockChange(bc)
		}
		nChanges += len(cs)
	}

	switch {
	case options&AsyncOperation != 0:
		go s.syncBlockChain()
	case options&ScheduledOperation != 0:
		// do nothing, will be synced later
	default:
		if err := s.syncBlockChain(); err != nil {
			return core.Errorw("cannot synchronize blockchain for access changes", err)
		}
	}

	core.End("%d changes", nChanges)
	return nil
}

func (s *Bao) convertToChanges(group Group, accessChanges []AccessChange) ([]Change, error) {
	core.Start("staging %d access changes for group %s", len(accessChanges), group)

	current, err := s.GetUsers(group)
	if err != nil {
		return nil, core.Errorw("cannot get existing access for group %s", group, err)
	}
	needNewKey := len(current) == 0

	adminRight := s.UserPublicId == s.Author
	if !adminRight {
		if acc, ok := current[s.UserPublicId]; ok && acc&Admin > 0 {
			adminRight = true
		}
	}
	if !adminRight {
		return nil, core.Errorw("only the vault creator or an admin can change the access rights")
	}

	keysForScope, err := s.getKeysForScope(group)
	if err != nil {
		return nil, core.Errorw("cannot get keys for group %s", group, err)
	}

	var changes []Change
	for _, change := range accessChanges {
		currentAccess := current[change.UserId]
		if currentAccess == change.Access {
			continue
		}

		if currentAccess == 0 && len(keysForScope) > 0 {
			activeKeySet := ActiveKeySet{
				Group: group,
				Id:    change.UserId,
				Keys:  make(map[uint64][]byte),
			}
			for keyId, key := range keysForScope {
				encKey, encErr := security.EcEncrypt(change.UserId, key)
				if encErr != nil {
					return nil, core.Errorw("cannot encrypt key for user %s in group %s", change.UserId, group, encErr)
				}
				activeKeySet.Keys[keyId] = encKey
			}
			changes = append(changes, &activeKeySet)
		}

		changes = append(changes, &ChangeAccess{
			Group:  group,
			Access: change.Access,
			Id:     change.UserId,
		})
		needNewKey = needNewKey || change.Access == 0
		current[change.UserId] = change.Access
	}

	if needNewKey {
		keyId := core.SnowID() &^ (1 << 62)
		key := core.GenerateRandomBytes(32)
		if err := s.setKeyToDB(keyId, group, key); err != nil {
			return nil, err
		}

		recipients := make([]security.PublicID, 0, len(current))
		for id, access := range current {
			if access != 0 {
				recipients = append(recipients, id)
			}
		}

		if len(recipients) > 0 {
			addKey, err := s.createAddKey(group, recipients, keyId, key)
			if err != nil {
				return nil, core.Errorw("cannot create add key for group %s", group, err)
			}
			changes = append(changes, &addKey)
			keysForScope = map[uint64]security.AESKey{keyId: key}
		}
	}

	core.Info("successfully created %d changes for group %s", len(changes), group)
	core.End("")
	return changes, nil
}

func (s *Bao) createAddKey(group Group, ids []security.PublicID, keyId uint64, key security.AESKey) (AddKey, error) {
	core.Start("creating add key for group %s, keyId %d", group, keyId)
	// Create a AddKey object with the given group, keyId, and key
	addKey := AddKey{
		Group:         group,
		KeyId:         keyId,
		EncryptedKeys: make(map[security.PublicID][]byte),
	}
	// Populate the EncodedKeys map with the new access rights
	for _, id := range ids {
		ekey, err := security.EcEncrypt(id, key)
		if err != nil {
			return AddKey{}, core.Errorw("cannot encrypt key for user %s in group %s", id, group, err)
		}
		addKey.EncryptedKeys[id] = ekey
	}
	core.End("created add key for group %s, keyId %d", group, keyId)
	return addKey, nil
}

// // requiresNewKey checks if at least one user has all accesses removed
// func (s *Bao) requiresNewKey(access Access) bool {
// 	for _, a := range access {
// 		if a == 0 {
// 			logrus.Infof("check if new key is required: user %s has all accesses removed", a)
// 			return true
// 		}
// 	}
// 	core.Info("check if new key is required: no new key needed")
// 	return false
// }

// GetUsers retrieves the access rights for a given group name.
func (s *Bao) GetUsers(group Group) (Accesses, error) {
	core.Start("group %s", group)
	var accesses Accesses = make(Accesses)

	rows, err := s.DB.Query("GET_USERS", sqlx.Args{"store": s.Id, "group": group})
	if err == sqlx.ErrNoRows {
		core.End("no users")
		// no users found, return empty access
		return accesses, nil
	}
	if err != nil {
		return nil, core.Errorw("cannot get users for group %s", group, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id security.PublicID
		var access Access
		err = rows.Scan(&id, &access)
		if err != nil {
			return nil, core.Errorw("cannot scan user %s for group %s", id, group, err)
		}
		accesses[id] = access
	}
	core.End("%d users", len(accesses))
	return accesses, nil
}

// GetGroups returns the groups and related access for a user
func (s *Bao) GetGroups(publicId security.PublicID) (map[Group]Access, error) {
	core.Start("user %s", publicId)
	var accesses map[Group]Access = make(map[Group]Access)

	rows, err := s.DB.Query("GET_GROUPS", sqlx.Args{"store": s.Id, "publicId": publicId})
	if err == sqlx.ErrNoRows {
		core.End("no groups found for user %s", publicId)
		// no groups found, return empty access
		return accesses, nil
	}
	if err != nil {
		return nil, core.Errorw("cannot get groups for user %s", publicId, err)
	}
	defer rows.Close()
	for rows.Next() {
		var group Group
		var access Access
		err = rows.Scan(&group, &access)
		if err != nil {
			return nil, core.Errorw("cannot scan group %s for user %s", group, publicId, err)
		}
		accesses[group] = access
	}

	core.End("%d groups", len(accesses))
	return accesses, nil
}
