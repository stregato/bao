package vault

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

//go:embed ddl1_0.sql
var ddl1_0 string

// Create creates a new Bao instance.
func Create(realm Realm, userPrivateID security.PrivateID, store store.Store, db *sqlx.DB, config Config) (*Vault, error) {
	core.Start("creating vault for url %s", store.ID())
	err := db.Define(ddl1_0)
	if err != nil {
		return nil, core.Errorw("Cannot define SQLite db in %s", db.DbPath, err)
	}

	userID, err := userPrivateID.PublicID()
	if err != nil {
		return nil, core.Errorw("invalid private while creating vault for url %s", store.ID(), err)
	}

	err = Wipe(store, realm.String())
	if err != nil {
		return nil, core.Errorw("cannot wipe data in store %s", store.ID(), err)
	}
	userIDHash := core.Int64Hash(userID.Bytes())
	ioThrottle := core.DefaultIfZero(config.IoThrottle, 10) // Default to 10 concurrent I/O operations

	id := fmt.Sprintf("%s|%s", store.ID(), realm.String())
	s := Vault{
		ID:               id,
		Realm:            realm,
		UserID:           userPrivateID,
		UserPublicID:     userID,
		UserPublicIDHash: userIDHash,
		Author:           userID,
		DB:               db,
		Config:           config,
		store:            store,
		//		lastChangeScheduledFolders: make(map[string]bool),
		lastCleanupAt: time.Now(),
		ioThrottleCh:  make(chan struct{}, ioThrottle),
		ioScheduleMap: make(map[FileId]chan struct{}),
	}

	bc, err := marshalChange(&config)
	if err != nil {
		return nil, core.Errorw("cannot marshal config change for vault %s", id, err)
	}
	err = s.stageBlockChange(bc)
	if err != nil {
		return nil, core.Errorw("cannot stage config change for vault %s", id, err)
	}
	err = s.SyncAccess(0, AccessChange{userID, ReadWriteAdmin})
	if err != nil {
		return nil, core.Errorw("cannot set access for vault %s", id, err)
	}

	go s.startHousekeeping()
	openedStashesMu.Lock()
	openedStashes = append(openedStashes, &s)
	openedStashesMu.Unlock()

	core.Info("Successfully created vault %s", id)
	return &s, nil
}

// wipe deletes all data from the store recursively.
func Wipe(s store.Store, dir string) error {
	ls, err := s.ReadDir(dir, store.Filter{})
	if os.IsNotExist(err) {
		core.Info("Store is empty, nothing to wipe")
		return nil
	}
	if err != nil {
		return core.Errorw("cannot read store %s", s.ID(), err)
	}
	for _, f := range ls {
		if err := s.Delete(path.Join(dir, f.Name())); err != nil {
			return core.Errorw("cannot delete file %s from store %s", f.Name(), s.ID(), err)
		}
	}
	logrus.Infof("Successfully wiped data from store %s", s.ID())
	return nil
}
