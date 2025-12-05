package bao

import (
	"path"
	"sort"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

func (s *Bao) deleteFilesBeforeModTime(threshold time.Time) (int64, error) {
	result, err := s.DB.Exec("DELETE_FILES_BEFORE_MODTIME", sqlx.Args{"store": s.Id, "modTime": threshold.UnixMilli()})
	if err != nil {
		return 0, core.Errorw("cannot delete files before modTime %s", threshold, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, core.Errorw("cannot read affected rows for retention cleanup", err)
	}
	return rows, nil
}

func (s *Bao) calculateAllocatedSize() (int64, error) {
	var total int64
	err := s.DB.QueryRow("CALCULATE_ALLOCATED_SIZE", sqlx.Args{"store": s.Id}, &total)
	if err != nil {
		return 0, core.Errorw("cannot calculate allocated size", err)
	}
	return total, nil
}

func (s *Bao) retentionCleanup() {
	store := s.store
	retention := s.Config.Retention
	if retention <= 0 {
		retention = DefaultRetention
	}

	retentionThreshold := core.Now().Add(-retention)

	var deletedDirs int
	var deletedRecords int64

	groups, err := s.ListGroups()
	if err != nil {
		core.LogError("cannot get groups for housekeeping: %v", err)
		return
	}

	for _, group := range groups {
		baseDir := path.Join(DataFolder, string(group))
		ls, _ := store.ReadDir(baseDir, storage.Filter{})
		sort.Slice(ls, func(i, j int) bool {
			return ls[i].Name() > ls[j].Name()
		})
		for _, l := range ls {
			if !l.IsDir() {
				continue
			}
			dir := path.Join(baseDir, l.Name())
			timestamp, err := time.Parse("20060102150405", l.Name())
			if err != nil {
				core.LogError("cannot parse timestamp folder %s: %v", l.Name(), err)
				continue
			}
			if timestamp.Before(retentionThreshold) {
				storage.DeleteDir(store, dir)
				deletedDirs++
				continue
			}
		}
	}

	deletedRecords, err = s.deleteFilesBeforeModTime(retentionThreshold)
	if err != nil {
		core.LogError("cannot delete files before retention threshold: %v", err)
	}

	if total, err := s.calculateAllocatedSize(); err != nil {
		core.LogError("cannot recalculate allocated size after retention cleanup: %v", err)
	} else {
		s.allocatedSize = total
	}

	core.Info("housekeeping: deleted %d day folders and %d db records by retention threshold", deletedDirs, deletedRecords)
}

func (s *Bao) housekeeping() error {
	core.Start("starting housekeeping")

	if s.store == nil {
		store, err := storage.Open(s.Url)
		if err != nil {
			return core.Errorw("cannot open store %s during housekeeping", s.Url, err)
		}
		s.store = store
	}

	if time.Since(s.lastBlockChainSyncAt) > core.DefaultIfZero(s.Config.BlockChainSyncPeriod, time.Hour) {
		s.syncBlockChain()
		s.lastBlockChainSyncAt = time.Now()
	}
	if time.Since(s.lastDirsSyncAt) > core.DefaultIfZero(s.Config.SyncPeriod, 24*time.Hour) {
		groups, err := s.ListGroups()
		if err != nil {
			core.LogError("cannot get groups for housekeeping: %v", err)
		}
		s.syncDirs(groups)
		s.lastDirsSyncAt = time.Now()
	}
	if time.Since(s.lastFilesSyncAt) > core.DefaultIfZero(s.Config.FilesSyncPeriod, 10*time.Minute) {
		s.waitFiles()
		s.lastFilesSyncAt = time.Now()
	}
	if time.Since(s.lastCleanupAt) > core.DefaultIfZero(s.Config.CleanupPeriod, 24*time.Hour) {
		s.retentionCleanup()
		s.lastCleanupAt = time.Now()
	}

	core.End("housekeeping completed")
	return nil
}

func (s *Bao) startHousekeeping() {
	core.Start("starting housekeeping")

	period := min(core.DefaultIfZero(s.Config.BlockChainSyncPeriod, time.Hour),
		core.DefaultIfZero(s.Config.SyncPeriod, 24*time.Hour),
		core.DefaultIfZero(s.Config.FilesSyncPeriod, 10*time.Minute),
		core.DefaultIfZero(s.Config.CleanupPeriod, 24*time.Hour))
	s.housekeepingTicker = time.NewTicker(period) // minimum of all periods
	go func() {
		for range s.housekeepingTicker.C {
			s.housekeeping()
		}
	}()
	core.End("housekeeping started")
}
