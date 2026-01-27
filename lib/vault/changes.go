package vault

import (
	"fmt"
	"strings"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/vmihailenco/msgpack/v5"
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
	Apply(s *Vault, author security.PublicID) error
}

// ActiveKeySet represents a the possible keys in the group given the retention period encoded with the user's public ID.
type ActiveKeySet struct {
	Id   security.PublicID
	Keys map[uint64][]byte // Key ID to encrypted key mapping
}

type ChangeAccess struct {
	Access   Access            `json:"access"`   // The new access level for the group
	PublicID security.PublicID `json:"publicId"` // User public ID whose access is being changed
}

func (c Config) Apply(s *Vault, author security.PublicID) error {
	core.Start("applying Config by author %s", author)
	return nil
}

func (c Config) String() string {
	return fmt.Sprintf("Config: retention=%v, maxStorage=%d, segmentInterval=%v, syncCooldown=%v, waitTimeout=%v, filesSyncPeriod=%v, cleanupPeriod=%v, blockChainSyncPeriod=%v, ioThrottle=%d",
		c.Retention,
		c.MaxStorage,
		c.SegmentInterval,
		c.SyncCooldown,
		c.WaitTimeout,
		c.FilesSyncPeriod,
		c.CleanupPeriod,
		c.BlockChainSyncPeriod,
		c.IoThrottle)
}

// AddKey represents a new key to be added to a specific group.
type AddKey struct {
	KeyId         uint64
	EncryptedKeys map[security.PublicID][]byte // Keys encrypted with the user's public key. Null if no new key is required.
}

// AddAttribute represents an attribute to be added to the vault.
type AddAttribute struct {
	Name  string // Attribute name
	Value string // Attribute value
}

func (v *Vault) stageBlockChange(blockChange BlockChange) error {
	core.Start("type %s", changeTypeLabels[blockChange.Type])
	_, err := v.DB.Exec("INSERT_STAGED_CHANGE", sqlx.Args{
		"vault":      v.ID,
		"changeType": blockChange.Type,
		"change":     blockChange.Payload,
	})
	if err != nil {
		return core.Errorw("cannot add change to the database", err)
	}
	core.End("staged %s to the database", changeTypeLabels[blockChange.Type])
	return nil
}

func (v *Vault) getStagedChanges() ([]BlockChange, error) {
	core.Start("vault %s", v.ID)
	var changes []BlockChange
	rows, err := v.DB.Query("GET_STAGED_CHANGES", sqlx.Args{"vault": v.ID})
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

func (a *AddAttribute) Apply(s *Vault, author security.PublicID) error {
	core.Start("name %s, value %s, author %s", a.Name, a.Value, author)

	_, err := s.DB.Exec("SET_ATTRIBUTE", sqlx.Args{
		"vault": s.ID,
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

func (a *AddAttribute) String() string {
	return fmt.Sprintf("AddAttribute: name=%s, value=%s", a.Name, a.Value)
}

func (v *Vault) addKey(keyId uint64, key []byte) error {
	core.Start("key %d, vault %s", keyId, v.Realm)
	if keyId == 0 {
		core.End("key ID is zero, no key to add")
		return nil
	}
	key, err := security.EcDecrypt(v.UserID, key)
	if err != nil {
		return core.Errorw("cannot decrypt key. My id is %s", v.UserPublicID, err)
	}
	err = v.setKeyToDB(keyId, key)
	if err != nil {
		return core.Errorw("cannot add key %d in vault %s", keyId, v.ID, err)
	}
	core.End("")
	return nil
}

func (a *AddKey) Apply(v *Vault, author security.PublicID) error {
	core.Start("applying AddKey by author %s", author)

	access, err := v.GetAccess(author)
	if err != nil {
		return core.Errorw("cannot get access for author %s", author, err)
	}
	var foundKeyForMe bool
	if access&Admin != 0 {
		for publicId, encodedKey := range a.EncryptedKeys {
			if publicId == v.UserPublicID {
				err = v.addKey(a.KeyId, encodedKey)
				if err != nil {
					return core.Errorw("cannot add key %d in vault %s", a.KeyId, v.ID, err)
				}
				foundKeyForMe = true
			}
		}
	}

	core.End("%d keys, key for me: %t", len(a.EncryptedKeys), foundKeyForMe)
	return nil
}

func (a *AddKey) String() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "AddKey: keyId=%d,  users=", a.KeyId)
	for id := range a.EncryptedKeys {
		fmt.Fprintf(&buf, "%x ", id.Hash())
	}
	return buf.String()
}

func (a *ActiveKeySet) Apply(v *Vault, author security.PublicID) error {
	core.Start("handling ActiveKeySet by author %s", author)

	if a.Id != v.UserPublicID {
		core.End("%d keys, not for me", len(a.Keys))
		return nil // Not for me
	}
	access, err := v.GetAccess(author)
	if err != nil {
		return core.Errorw("cannot get access for author %s", author, err)
	}
	if access&Admin != 0 {
		for keyId, encodedKey := range a.Keys {
			v.addKey(keyId, encodedKey)
		}
	}
	core.End("%d keys for me", len(a.Keys))
	return nil
}

func (a *ActiveKeySet) String() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "ActiveKeySet: user: %x, key ids: ", a.Id.Hash())
	for keyId := range a.Keys {
		fmt.Fprintf(&buf, "%x, ", keyId)
	}
	return buf.String()
}

func (c *ChangeAccess) Apply(v *Vault, author security.PublicID) error {
	core.Start("")

	adminRight := author == v.Author

	if !adminRight {
		access, err := v.GetAccess(author)
		if err != nil && err != sqlx.ErrNoRows {
			return core.Errorw("cannot get access for author %s", author, err)
		}
		adminRight = access&Admin != 0
	}

	if adminRight {
		if c.Access == 0 {
			// Remove user if access is zero
			err := v.removeUser(c.PublicID)
			if err != nil {
				return core.Errorw("cannot remove user %s from vault %s", c.PublicID, v.ID, err)
			}
			if v.UserPublicID == c.PublicID {
				core.Info("my access to vault %s removed", v.ID)
			}
		} else {
			// Set user access
			err := v.setUser(c.PublicID, c.Access)
			if err != nil {
				return core.Errorw("cannot set user %s access for vault %s", c.PublicID, v.ID, err)
			}
			if v.UserPublicID == c.PublicID {
				core.Info("my access for vault %s changed to %s", v.ID, AccessLabels[c.Access])
			}
		}
	} else {
		core.Info("author %s does not have admin rights to change access in vault %s", author, v.ID)
	}
	core.End("handled access change for vault %s to %s: id %s", v.ID, AccessLabels[c.Access],
		c.PublicID)
	return nil
}

func (ca ChangeAccess) String() string {
	return fmt.Sprintf("ChangeAccess: userID=%x, access=%s", ca.PublicID.Hash(), AccessLabels[ca.Access])
}
