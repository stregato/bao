package bao

import (
	_ "embed"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

//go:embed ddl1_0.sql
var ddl1_0 string

// Create creates a new Bao instance.
func Create(db *sqlx.DB, user security.PrivateID, StoreManifest storage.StoreConfig, config Config) (*Bao, error) {
	core.Start("creating vault for url %s", StoreManifest.Id)
	err := db.Define(ddl1_0)
	if err != nil {
		return nil, core.Errorw("Cannot define SQLite db in %s", db.DbPath, err)
	}

	publicId, err := user.PublicID()
	if err != nil {
		return nil, core.Errorw("invalid private while creating vault for url %s", StoreManifest.Id, err)
	}

	store, err := storage.Open(StoreManifest)
	if err != nil {
		return nil, core.Errorw("cannot open store while creating vault for url %s", StoreManifest.Id, err)
	}

	err = wipeData(store)
	if err != nil {
		return nil, core.Errorw("cannot wipe data in store %s", StoreManifest.Id, err)
	}
	publicIdHash := core.Int64Hash(publicId.Bytes())
	ioThrottle := core.DefaultIfZero(config.IoThrottle, 10) // Default to 10 concurrent I/O operations

	s := Bao{
		Id:               StoreManifest.Id,
		UserId:           user,
		UserPublicId:     publicId,
		UserPublicIdHash: publicIdHash,
		Author:           publicId,
		StoreConfig:      StoreManifest,
		DB:               db,
		Config:           config,
		store:            store,
		//		lastChangeScheduledFolders: make(map[string]bool),
		lastCleanupAt:   time.Now(),
		ioThrottleCh:    make(chan struct{}, ioThrottle),
		ioScheduleMap:   make(map[FileId]chan struct{}),
		ioWritingWgMaps: make(map[Group]*sync.WaitGroup),
	}

	bc, err := marshalChange(&config)
	if err != nil {
		return nil, core.Errorw("cannot marshal config change for vault %s", StoreManifest.Id, err)
	}
	err = s.stageBlockChange(bc)
	if err != nil {
		return nil, core.Errorw("cannot stage config change for vault %s", StoreManifest.Id, err)
	}

	err = s.SyncAccess(0, AccessChange{Group: Admins, Access: ReadWrite, UserId: user.PublicIDMust()})
	if err != nil {
		return nil, core.Errorw("cannot set access for vault %s", StoreManifest.Id, err)
	}

	go s.startHousekeeping()
	openedStashesMu.Lock()
	openedStashes = append(openedStashes, &s)
	openedStashesMu.Unlock()

	core.Info("Successfully created Bao instance for url %s: %s", StoreManifest.Id, s.String())
	return &s, nil
}

// wipeData deletes all data from the store recursively.
func wipeData(store storage.Store) error {
	ls, err := store.ReadDir("", storage.Filter{})
	if os.IsNotExist(err) {
		core.Info("Store is empty, nothing to wipe")
		return nil
	}
	if err != nil {
		return core.Errorw("cannot read store %s", store.ID(), err)
	}
	for _, f := range ls {
		if err := store.Delete(f.Name()); err != nil {
			return core.Errorw("cannot delete file %s from store %s", f.Name(), store.ID(), err)
		}
	}
	logrus.Infof("Successfully wiped data from store %s", store.ID())
	return nil
}
