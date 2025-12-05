package bao

import (
	"path"
	"strings"
	"sync"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/vmihailenco/msgpack/v5"
)

type OpenOption int

const (
	Sync OpenOption = 1 << iota // Sync indicates that the operation should be performed synchronously, waiting for completion
)

func Open(db *sqlx.DB, user security.PrivateID, url string, author security.PublicID) (*Bao, error) {
	core.Start("opening Bao instance with store URL %s", url)
	err := db.Define(ddl1_0)
	if err != nil {
		return nil, core.Errorw("Cannot define SQLite db in %s", db.DbPath, err)
	}

	lastSlash := strings.LastIndex(url, "/")
	if lastSlash == -1 {
		return nil, core.Errorw("Invalid store URL: %s", url)
	}

	publicId, err := user.PublicID()
	if err != nil {
		return nil, core.Errorw("cannot get public ID from private ID %s", user, err)
	}

	id := core.StringHash(append([]byte(url), publicId.Bytes()...))
	var config Config
	_, _, _, b, _ := db.GetSetting(path.Join("/bao/config/", id))
	configAvailable := (b != nil) && (msgpack.Unmarshal(b, &config) == nil)

	ioThrottle := core.DefaultIfZero(config.IoThrottle, 10) // Default to 10 concurrent I/O operations

	s := Bao{
		Id:               core.StringHash(append([]byte(url), publicId.Bytes()...)),
		UserId:           user,
		UserPublicId:     publicId,
		UserPublicIdHash: core.Int64Hash(publicId.Bytes()),
		Author:           author,
		DB:               db,
		Url:              url,
		//		lastChangeScheduledFolders: make(map[string]bool),
		lastCleanupAt:   time.Now(),
		ioThrottleCh:    make(chan struct{}, ioThrottle),
		ioScheduleMap:   make(map[FileId]chan struct{}),
		ioWritingWgMaps: make(map[Group]*sync.WaitGroup),
	}
	allocatedSize, err := s.calculateAllocatedSize()
	if err != nil {
		return nil, core.Errorw("cannot calculate allocated size for stash %s", url, err)
	}
	s.allocatedSize = allocatedSize
	s.Config = config
	// if it is the first time we open the stash, better synchronize the users
	if !configAvailable {
		err := s.syncBlockChain()
		if err != nil {
			if s.store != nil {
				_ = s.store.Close() // Close the store if it was opened
			}
			return nil, core.Errorw("cannot perform initial user synchronization for stash %s", url, err)
		}
	}

	go s.ListGroups()
	s.startHousekeeping()

	core.Info("successfully opened Bao instance with store URL %s", s.Url)
	core.End("")
	return &s, nil
}
