package bao

import (
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/vmihailenco/msgpack/v5"
	"gopkg.in/yaml.v2"
)

type ChangeType uint8

const (
	config       ChangeType = iota // Changing settings for the vault
	activeKeySet                   // Active key set for a specific group
	changeAccess                   // Change access for all users in the group
	addKey                         // Add a new key for a specific group
	addAttribute                   // Add a new attribute to the vault
)

var changeTypeLabels = []string{
	"config",
	"activeKeySet",
	"changeAccess",
	"addKey",
	"addAttribute",
}

type Change interface {
	Apply(s *Bao, author security.PublicID) error
}

// ActiveKeySet represents a the possible keys in the group given the retention period encoded with the user's public ID.
type ActiveKeySet struct {
	Group Group
	Id    security.PublicID
	Keys  map[uint64][]byte // Key ID to encrypted key mapping
}

func (aks ActiveKeySet) String() string {
	y, _ := yaml.Marshal(aks)
	return string(y)
}

type ChangeAccess struct {
	Group  Group             // The group for which access is being changed
	Access Access            // The new access level for the group
	Id     security.PublicID // User public ID whose access is being changed
}

func (ca ChangeAccess) String() string {
	y, _ := yaml.Marshal(ca)
	return string(y)
}

type Config struct {
	Retention            time.Duration `json:"retention"`            // How long data is kept
	MaxStorage           int64         `json:"maxStorage"`           // Maximum allowed storage (bytes)
	SegmentInterval      time.Duration `json:"segmentInterval"`      // Time duration of each batch segment
	SyncTimeout          time.Duration `json:"syncTimeout"`          // Timeout for sync operations (default 10 minutes)
	SyncPeriod           time.Duration `json:"dirsSyncPeriod"`       // How often to sync the vault (default no sync)
	FilesSyncPeriod      time.Duration `json:"filesSyncPeriod"`      // How often to sync files (default 10 minutes)
	CleanupPeriod        time.Duration `json:"cleanupPeriod"`        // How often to run housekeeping (default 1 hour)
	BlockChainSyncPeriod time.Duration `json:"blockChainSyncPeriod"` // How often to sync the blockchain (default 10 minutes)
	IoThrottle           int64         `json:"ioThrottle"`           // Maximum number of concurrent I/O operations. Default is 10.
	ChainRepair          int           `json:"chainRepair"`          // Number of blocks to keep for chain repair
}

func (c Config) Apply(s *Bao, author security.PublicID) error {
	core.Start("applying Config by author %s", author)
	return nil
}

// AddKey represents a new key to be added to a specific group.
type AddKey struct {
	Group         Group
	KeyId         uint64
	EncryptedKeys map[security.PublicID][]byte // Keys encrypted with the user's public key. Null if no new key is required.
}

// AddAttribute represents an attribute to be added to the vault.
type AddAttribute struct {
	Name  string // Attribute name
	Value string // Attribute value
}

func (s *Bao) stageBlockChange(blockChange BlockChange) error {
	core.Start("type %s", changeTypeLabels[blockChange.Type])
	_, err := s.DB.Exec("INSERT_STAGED_CHANGE", sqlx.Args{
		"store":      s.Id,
		"changeType": blockChange.Type,
		"change":     blockChange.Payload,
	})
	if err != nil {
		return core.Errorw("cannot add change to the database", err)
	}
	core.End("staged %s to the database", changeTypeLabels[blockChange.Type])
	return nil
}

func (s *Bao) getStagedChanges() ([]BlockChange, error) {
	core.Start("vault %s", s.Id)
	var changes []BlockChange
	rows, err := s.DB.Query("GET_STAGED_CHANGES", sqlx.Args{"store": s.Id})
	if err != nil {

		return nil, core.Errorw("cannot get staged changes", err)
	}
	defer rows.Close()

	for rows.Next() {
		var change BlockChange
		if err := rows.Scan(&change.Type, &change.Payload); err != nil {
			return nil, core.Errorw("cannot scan staged change", err)
		}
		changes = append(changes, change)
	}
	core.End("%d changes", len(changes))
	return changes, nil
}

func unmarshalChange(blockChange BlockChange) (Change, error) {
	core.Trace("type %s", changeTypeLabels[blockChange.Type])
	defer core.Trace("unmarshaled change of type %s", changeTypeLabels[blockChange.Type])
	var err error
	var change Change
	switch blockChange.Type {
	case activeKeySet:
		var aks ActiveKeySet
		err = msgpack.Unmarshal(blockChange.Payload, &aks)
		change = &aks
	case changeAccess:
		var ca ChangeAccess
		err = msgpack.Unmarshal(blockChange.Payload, &ca)
		change = &ca
	case addKey:
		var ak AddKey
		err = msgpack.Unmarshal(blockChange.Payload, &ak)
		change = &ak
	case addAttribute:
		var aa AddAttribute
		err = msgpack.Unmarshal(blockChange.Payload, &aa)
		change = &aa
	case config:
		var c Config
		err = msgpack.Unmarshal(blockChange.Payload, &c)
		change = &c
	default:
		return nil, core.Errorw("unknown change type: %d", blockChange.Type)
	}
	if err != nil {
		return nil, core.Errorw("cannot unmarshal change of type %s", changeTypeLabels[blockChange.Type], err)
	}
	return change, nil
}

func marshalChange(change Change) (BlockChange, error) {
	core.Start("type %T", change)

	payload, err := msgpack.Marshal(change)
	if err != nil {
		return BlockChange{}, core.Errorw("cannot marshal change", err)
	}
	defer core.End("")

	switch change := change.(type) {
	case *ActiveKeySet:
		return BlockChange{activeKeySet, payload}, nil
	case *ChangeAccess:
		return BlockChange{changeAccess, payload}, nil
	case *AddKey:
		return BlockChange{addKey, payload}, nil
	case *AddAttribute:
		return BlockChange{addAttribute, payload}, nil
	case *Config:
		return BlockChange{config, payload}, nil
	default:
		return BlockChange{}, core.Errorw("unknown change type: %T", change)
	}
}

func (a *AddAttribute) Apply(s *Bao, author security.PublicID) error {
	core.Start("name %s, value %s, author %s", a.Name, a.Value, author)

	_, err := s.DB.Exec("SET_ATTRIBUTE", sqlx.Args{
		"store": s.Id,
		"name":  a.Name,
		"value": a.Value,
		"id":    author,
		"tm":    core.Now().Unix(),
	})
	if err != nil {
		return core.Errorw("cannot add attribute %s for id %s", a.Name, author, err)
	}

	core.End("")
	return nil
}

// func (s *Bao) handleChange(author security.PublicID, blockChange BlockChange) error {
// 	core.Start("handling change %s by author %s", changeTypeLabels[blockChange.Type], author)

// 	var err error
// 	switch blockChange.Type {
// 	case settings:
// 		// Handle settings change
// 		var settings Config
// 		// Only the vault author can change settings
// 		if author == s.Author {
// 			err := msgpack.Unmarshal(blockChange.Payload, &settings)
// 			if err != nil {
// 				return core.Errorw("cannot unmarshal settings change", err)
// 			}
// 			s.Config = settings
// 		}
// 	case changeAccess:
// 		var ChangeAccess ChangeAccess
// 		err = msgpack.Unmarshal(blockChange.Payload, &ChangeAccess)
// 	case addKey:
// 		err = s.handleAddKey(author, blockChange)
// 	case activeKeySet:
// 		err = s.handleActiveKeySet(author, blockChange)
// 	case addAttribute:
// 		err = s.handleAddAttribute(author, blockChange)
// 	default:
// 		err = core.Errorw("unknown change type: %d", blockChange.Type)
// 	}
// 	if err != nil {
// 		return core.Errorw("cannot handle change of type %d by author %s", blockChange.Type, author, err)
// 	}
// 	core.Info("access change of type %s", changeTypeLabels[blockChange.Type])
// 	core.End("")
// 	return err
// }

func (s *Bao) hasAdminAccess(id security.PublicID, group Group) (bool, error) {
	core.Start("checking admin access for user %s in group %s", id, group)
	if id == s.Author {
		core.End("user %s is author of vault", id)
		return true, nil // Author always has admin access
	}

	accesses, err := s.GetUsers(group)
	if err != nil {
		return false, core.Errorw("cannot get access for group %s", group, err)
	}
	if access, ok := accesses[id]; ok && access&Admin != 0 {
		core.End("user %s is admin in group %s", id, group)
		return true, nil
	}
	admins, err := s.GetUsers(Admins)
	if err != nil {
		return false, core.Errorw("cannot get admins", err)
	}
	if _, ok := admins[id]; ok {
		core.End("user %s is admin in group %s", id, group)
		return true, nil
	}
	core.End("user %s does not have admin access in group %s", id, group)
	return false, nil
}

func (s *Bao) addKey(group Group, keyId uint64, key []byte) error {
	core.Start("adding key %d for group %s", keyId, group)
	if keyId == 0 {
		core.End("key ID is zero, no key to add")
		return nil
	}
	key, err := security.EcDecrypt(s.UserId, key)
	if err != nil {
		return core.Errorw("cannot decrypt key. My id is %s", s.UserId, err)
	}
	err = s.setKeyToDB(keyId, group, key)
	if err != nil {
		return core.Errorw("cannot add key %d for group %s", keyId, group, err)
	}
	core.End("")
	return nil
}

func (a *AddKey) Apply(s *Bao, author security.PublicID) error {
	core.Start("applying AddKey by author %s", author)

	hasAdminAccess, err := s.hasAdminAccess(author, a.Group)
	if err != nil {
		return core.Errorw("cannot check admin access for author %s in group %s", author, a.Group, err)
	}
	var foundKeyForMe bool
	if hasAdminAccess {
		for publicId, encodedKey := range a.EncryptedKeys {
			if publicId == s.UserPublicId {
				err = s.addKey(a.Group, a.KeyId, encodedKey)
				if err != nil {
					return core.Errorw("cannot add key %d for group %s", a.KeyId, a.Group, err)
				}
				foundKeyForMe = true
			}
		}
	}

	core.End("%d keys, key for me: %t", len(a.EncryptedKeys), foundKeyForMe)
	return nil
}

// func (s *Bao) handleAddKey(author security.PublicID, change BlockChange) error {
// 	core.Start("handling AddKey by author %s", author)
// 	var addKey AddKey
// 	err := msgpack.Unmarshal(change.Payload, &addKey)
// 	if err != nil {
// 		return core.Errorw("cannot unmarshal add key change", err)
// 	}

// 	hasAdminAccess, err := s.hasAdminAccess(author, addKey.Group)
// 	if err != nil {
// 		return core.Errorw("cannot check admin access for author %s in group %s", author, addKey.Group, err)
// 	}
// 	var foundKeyForMe bool
// 	if hasAdminAccess {
// 		for publicId, encodedKey := range addKey.EncryptedKeys {
// 			if publicId == s.UserPublicId {
// 				err = s.addKey(addKey.Group, addKey.KeyId, encodedKey)
// 				if err != nil {
// 					return core.Errorw("cannot add key %d for group %s", addKey.KeyId, addKey.Group, err)
// 				}
// 				foundKeyForMe = true
// 			}
// 		}
// 	}
// 	core.End("%d keys, key for me: %t", len(addKey.EncryptedKeys), foundKeyForMe)
// 	return nil
// }

func (a *ActiveKeySet) Apply(s *Bao, author security.PublicID) error {
	core.Start("handling ActiveKeySet by author %s", author)

	if a.Id != s.UserPublicId {
		core.End("%d keys, not for me", len(a.Keys))
		return nil // Not for me
	}
	hasAdminAccess, err := s.hasAdminAccess(author, a.Group)
	if err != nil {
		return core.Errorw("cannot check admin access for author %s in group %s", author, a.Group, err)
	}
	if hasAdminAccess {
		for keyId, encodedKey := range a.Keys {
			s.addKey(a.Group, keyId, encodedKey)
		}
	}
	core.End("%d keys for me", len(a.Keys))
	return nil
}

// func (s *Bao) handleActiveKeySet(author security.PublicID, change BlockChange) error {
// 	core.Start("handling ActiveKeySet by author %s", author)
// 	var activeKeySet ActiveKeySet
// 	err := msgpack.Unmarshal(change.Payload, &activeKeySet)
// 	if err != nil {
// 		return core.Errorw("cannot unmarshal active key set change", err)
// 	}

// 	if activeKeySet.Id != s.UserPublicId {
// 		core.End("%d keys, not for me", len(activeKeySet.Keys))
// 		return nil // Not for me
// 	}
// 	hasAdminAccess, err := s.hasAdminAccess(author, activeKeySet.Group)
// 	if err != nil {
// 		return core.Errorw("cannot check admin access for author %s in group %s", author, activeKeySet.Group, err)
// 	}
// 	if hasAdminAccess {
// 		for keyId, encodedKey := range activeKeySet.Keys {
// 			s.addKey(activeKeySet.Group, keyId, encodedKey)
// 		}
// 	}
// 	core.End("%d keys for me", len(activeKeySet.Keys))
// 	return nil
// }

func (c *ChangeAccess) Apply(s *Bao, author security.PublicID) error {
	core.Start("")
	hasAdminAccess, err := s.hasAdminAccess(author, c.Group)
	if err != nil {
		return core.Errorw("cannot check admin access for author %s in group %s", author, c.Group, err)
	}

	if hasAdminAccess {
		if c.Access == 0 {
			// Remove user if access is zero
			err = s.removeUser(c.Group, c.Id)
			if err != nil {
				return core.Errorw("cannot remove user %s from group %s", c.Id, c.Group, err)
			}
			if s.UserPublicId == c.Id {
				core.Info("my access for group %s removed", c.Group)
			}
		} else {
			// Set user access
			err = s.setUser(c.Group, c.Id, c.Access)
			if err != nil {
				return core.Errorw("cannot set user %s access for group %s", c.Id, c.Group, err)
			}
			if s.UserPublicId == c.Id {
				core.Info("my access for group %s changed to %s", c.Group, AccessLabels[c.Access])
			}
		}
	}
	core.End("handled access change for group %s to %s: id %s", c.Group, AccessLabels[c.Access],
		c.Id)
	return nil
}

// func (s *Bao) handleChangeAccess(author security.PublicID, change BlockChange) error {
// 	core.Start("")
// 	var changeAccess ChangeAccess
// 	err := msgpack.Unmarshal(change.Payload, &changeAccess)
// 	if err != nil {
// 		return core.Errorw("cannot unmarshal change access", err)
// 	}

// 	hasAdminAccess, err := s.hasAdminAccess(author, changeAccess.Group)
// 	if err != nil {
// 		return core.Errorw("cannot check admin access for author %s in group %s", author, changeAccess.Group, err)
// 	}

// 	if hasAdminAccess {
// 		publicId := changeAccess.Id
// 		if changeAccess.Access == 0 {
// 			// Remove user if access is zero
// 			err = s.removeUser(changeAccess.Group, publicId)
// 			if err != nil {
// 				return core.Errorw("cannot remove user %s from group %s", publicId, changeAccess.Group, err)
// 			}
// 			if s.UserPublicId == publicId {
// 				core.Info("my access for group %s removed", changeAccess.Group)
// 			}
// 		} else {
// 			// Set user access
// 			err = s.setUser(changeAccess.Group, publicId, changeAccess.Access)
// 			if err != nil {
// 				return core.Errorw("cannot set user %s access for group %s", publicId, changeAccess.Group, err)
// 			}
// 			if s.UserPublicId == publicId {
// 				core.Info("my access for group %s changed to %s", changeAccess.Group, AccessLabels[changeAccess.Access])
// 			}
// 		}
// 	}
// 	core.End("handled access change for group %s to %s: id %s", changeAccess.Group, AccessLabels[changeAccess.Access],
// 		changeAccess.Id)
// 	return nil
// }
