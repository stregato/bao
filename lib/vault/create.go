package vault

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
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
func Create(realm Realm, userSecret security.PrivateID, store store.Store, db *sqlx.DB, config Config) (*Vault, error) {
	core.Start("creating vault for url %s", store.ID())
	err := db.Define(ddl1_0)
	if err != nil {
		return nil, core.Error(core.DbError, "Cannot define SQLite db in %s", db.DbPath, err)
	}

	if config.SyncRelay != "" && !strings.HasPrefix(config.SyncRelay, "ws") {
		return nil, core.Error(core.ConfigError, "Invalid watch service URL %s, must start with ws:// or wss://", config.SyncRelay)
	}

	userID, err := userSecret.PublicID()
	if err != nil {
		return nil, core.Error(core.GenericError, "invalid private while creating vault for url %s", store.ID(), err)
	}

	err = Wipe(store, realm.String())
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot wipe data in store %s", store.ID(), err)
	}
	userIDHash := core.Int64Hash(userID.Bytes())
	ioThrottle := core.DefaultIfZero(config.IoThrottle, 10) // Default to 10 concurrent I/O operations

	id := fmt.Sprintf("%s@%s", realm.String(), store.ID())
	v := Vault{
		ID:            id,
		Realm:         realm,
		UserSecret:    userSecret,
		UserID:        userID,
		UserIDHash:    userIDHash,
		Author:        userID,
		DB:            db,
		Config:        config,
		store:         store,
		newFiles:      sync.NewCond(&sync.Mutex{}),
		lastCleanupAt: time.Now(),
		ioThrottleCh:  make(chan struct{}, ioThrottle),
		ioScheduleMap: make(map[FileId]chan struct{}),
	}

	bc, err := marshalChange(&config)
	if err != nil {
		return nil, core.Error(core.ParseError, "cannot marshal config change for vault %s", id, err)
	}
	err = v.stageBlockChange(bc)
	if err != nil {
		return nil, core.Error(core.ConfigError, "cannot stage config change for vault %s", id, err)
	}
	err = v.SyncAccess(0, AccessChange{userID, ReadWriteAdmin})
	if err != nil {
		return nil, core.Error(core.DbError, "cannot set access for vault %s", id, err)
	}

	v.startHousekeeping()
	openedStashesMu.Lock()
	openedStashes = append(openedStashes, &v)
	openedStashesMu.Unlock()

	core.Info("Successfully created vault %s", id)
	return &v, nil
}

// wipe deletes all data from the store recursively.
func Wipe(s store.Store, dir string) error {
	ls, err := s.ReadDir(dir, store.Filter{})
	if os.IsNotExist(err) {
		core.Info("Store is empty, nothing to wipe")
		return nil
	}
	if err != nil {
		return core.Error(core.GenericError, "cannot read store %s", s.ID(), err)
	}
	for _, f := range ls {
		if err := s.Delete(path.Join(dir, f.Name())); err != nil {
			return core.Error(core.DbError, "cannot delete file %s from store %s", f.Name(), s.ID(), err)
		}
	}
	logrus.Infof("Successfully wiped data from store %s", s.ID())
	return nil
}
