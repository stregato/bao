package vault

import (
	"path"
	"sort"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

func (v *Vault) deleteFilesBeforeModTime(threshold time.Time) (int64, error) {
	result, err := v.DB.Exec("DELETE_FILES_BEFORE_MODTIME", sqlx.Args{"vault": v.ID, "modTime": threshold.UnixMilli()})
	if err != nil {
		return 0, core.Error(core.DbError, "cannot delete files before modTime %s", threshold, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, core.Error(core.DbError, "cannot read affected rows for retention cleanup", err)
	}
	return rows, nil
}

func (v *Vault) calculateAllocatedSize() (int64, error) {
	var total int64
	err := v.DB.QueryRow("CALCULATE_ALLOCATED_SIZE", sqlx.Args{"vault": v.ID}, &total)
	if err != nil {
		return 0, core.Error(core.GenericError, "cannot calculate allocated size", err)
	}
	return total, nil
}

func (v *Vault) retentionCleanup() {
	retention := v.Config.Retention
	if retention <= 0 {
		retention = DefaultRetention
	}

	retentionThreshold := core.Now().Add(-retention)

	var deletedDirs int
	var deletedRecords int64

	baseDir := path.Join(DataFolder, string(v.Realm))
	ls, _ := v.store.ReadDir(baseDir, store.Filter{})
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
			store.DeleteDir(v.store, dir)
			deletedDirs++
			continue
		}
	}

	deletedRecords, err := v.deleteFilesBeforeModTime(retentionThreshold)
	if err != nil {
		core.LogError("cannot delete files before retention threshold: %v", err)
	}

	if total, err := v.calculateAllocatedSize(); err != nil {
		core.LogError("cannot recalculate allocated size after retention cleanup: %v", err)
	} else {
		v.allocatedSize = total
	}

	core.Info("housekeeping: deleted %d day folders and %d db records by retention threshold", deletedDirs, deletedRecords)
}

func (v *Vault) housekeeping() error {
	core.Start("starting housekeeping")
	if time.Since(v.lastBlockChainSyncAt) > core.DefaultIfZero(v.Config.BlockChainSyncPeriod, time.Hour) {
		v.syncBlockChain()
		v.lastBlockChainSyncAt = time.Now()
	}
	if time.Since(v.lastWaitFilesAt) > core.DefaultIfZero(v.Config.FilesSyncPeriod, 10*time.Minute) {
		v.waitFiles()
		v.lastWaitFilesAt = time.Now()
	}
	if time.Since(v.lastCleanupAt) > core.DefaultIfZero(v.Config.CleanupPeriod, 24*time.Hour) {
		v.retentionCleanup()
		v.lastCleanupAt = time.Now()
	}

	core.End("housekeeping completed")
	return nil
}

func (v *Vault) startHousekeeping() {
	core.Start("starting housekeeping")

	period := min(core.DefaultIfZero(v.Config.BlockChainSyncPeriod, time.Hour),
		core.DefaultIfZero(v.Config.SyncCooldown, 24*time.Hour),
		core.DefaultIfZero(v.Config.FilesSyncPeriod, 10*time.Minute),
		core.DefaultIfZero(v.Config.CleanupPeriod, 24*time.Hour))
	v.housekeepingTicker = time.NewTicker(period) // minimum of all periods
	go func() {
		for range v.housekeepingTicker.C {
			v.housekeeping()
		}
	}()
	core.End("housekeeping started")
}
