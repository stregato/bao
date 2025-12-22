package bao

import (
	"path"
	"sync"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
	"github.com/vmihailenco/msgpack/v5"
)

type OpenOption int

const (
	Sync OpenOption = 1 << iota // Sync indicates that the operation should be performed synchronously, waiting for completion
)

func Open(db *sqlx.DB, user security.PrivateID, storeManifest storage.StoreConfig, author security.PublicID) (*Bao, error) {
	core.Start("opening Bao instance with store URL %s", storeManifest.Id)
	err := db.Define(ddl1_0)
	if err != nil {
		return nil, core.Errorw("Cannot define SQLite db in %s", db.DbPath, err)
	}

	publicId, err := user.PublicID()
	if err != nil {
		return nil, core.Errorw("cannot get public ID from private ID %s", user, err)
	}

	var config Config
	_, _, _, b, _ := db.GetSetting(path.Join("/bao/config/", storeManifest.Id))
	configAvailable := (b != nil) && (msgpack.Unmarshal(b, &config) == nil)

	ioThrottle := core.DefaultIfZero(config.IoThrottle, 10) // Default to 10 concurrent I/O operations

	s := Bao{
		Id:               storeManifest.Id,
		UserId:           user,
		UserPublicId:     publicId,
		UserPublicIdHash: core.Int64Hash(publicId.Bytes()),
		Author:           author,
		DB:               db,
		StoreConfig:      storeManifest,
		//		lastChangeScheduledFolders: make(map[string]bool),
		lastCleanupAt:   time.Now(),
		ioThrottleCh:    make(chan struct{}, ioThrottle),
		ioScheduleMap:   make(map[FileId]chan struct{}),
		ioWritingWgMaps: make(map[Group]*sync.WaitGroup),
	}
	allocatedSize, err := s.calculateAllocatedSize()
	if err != nil {
		return nil, core.Errorw("cannot calculate allocated size for vault %s", storeManifest.Id, err)
	}
	s.allocatedSize = allocatedSize
	s.Config = config
	// if it is the first time we open the vault, better synchronize the users
	if !configAvailable {
		err := s.syncBlockChain()
		if err != nil {
			if s.store != nil {
				_ = s.store.Close() // Close the store if it was opened
			}
			return nil, core.Errorw("cannot perform initial user synchronization for vault %s", storeManifest.Id, err)
		}
	}

	go s.ListGroups()
	s.startHousekeeping()

	core.Info("successfully opened Bao instance %s", s.Id)
	core.End("")
	return &s, nil
}
