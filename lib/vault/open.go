package vault

import (
	"fmt"
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
	"github.com/vmihailenco/msgpack/v5"
)

type OpenOption int

const (
	Sync OpenOption = 1 << iota // Sync indicates that the operation should be performed synchronously, waiting for completion
)

func Open(realm Realm, userPrivateID security.PrivateID, author security.PublicID, store store.Store, db *sqlx.DB) (*Vault, error) {
	core.Start("opening vault with store URL %s", store.ID())
	err := db.Define(ddl1_0)
	if err != nil {
		return nil, core.Errorw("Cannot define SQLite db in %s", db.DbPath, err)
	}

	userID, err := userPrivateID.PublicID()
	if err != nil {
		return nil, core.Errorw("cannot get public ID from private ID %s", userPrivateID, err)
	}

	var config Config
	id := fmt.Sprintf("%s|%s", store.ID(), realm.String())
	_, _, _, b, _ := db.GetSetting(path.Join("/bao/config/", id))
	if b != nil {
		err := msgpack.Unmarshal(b, &config)
		if err != nil {
			return nil, core.Errorw("cannot unmarshal config for vault %s", id, err)
		}
	}

	ioThrottle := core.DefaultIfZero(config.IoThrottle, 10) // Default to 10 concurrent I/O operations

	v := Vault{
		ID:               id,
		Realm:            realm,
		UserID:           userPrivateID,
		UserPublicID:     userID,
		UserPublicIDHash: core.Int64Hash(userID.Bytes()),
		Author:           author,
		DB:               db,
		Config:           config,
		store:            store,

		//		lastChangeScheduledFolders: make(map[string]bool),
		lastCleanupAt: time.Now(),
		ioThrottleCh:  make(chan struct{}, ioThrottle),
		ioScheduleMap: make(map[FileId]chan struct{}),
	}
	allocatedSize, err := v.calculateAllocatedSize()
	if err != nil {
		return nil, core.Errorw("cannot calculate allocated size for vault %s", id, err)
	}
	v.allocatedSize = allocatedSize

	access, err := v.GetAccess(v.UserPublicID)
	if err != nil {
		return nil, core.Errorw("cannot get access for user %s in vault %s", v.UserPublicID, id, err)
	}
	if access == 0 {
		err := v.syncBlockChain()
		if err != nil {
			return nil, core.Errorw("cannot perform initial user synchronization for vault %s", id, err)
		}
		access, err = v.GetAccess(v.UserPublicID)
		if err != nil {
			return nil, core.Errorw("Cannot get access for user %s in vault %s", v.UserPublicID, id, err)
		}
		if access == 0 {
			return nil, core.Errorw("Access Denied: user %s, vault %s", v.UserPublicID, id)
		}
	} else {
		defer v.syncBlockChain()
	}
	v.startHousekeeping()

	core.Info("successfully opened vault %s", v.ID)
	core.End("")
	return &v, nil
}
