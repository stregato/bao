package vault

import (
	"path"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/store"
)

func (v *Vault) Delete(name string, options IOOption) error {
	core.Start("name %s", name)
	now := core.Now()

	file, found, err := v.queryFileByName(name)
	if err != nil {
		return core.Error(core.DbError, "cannot query file %s", name, err)
	}
	if !found {
		return nil // File does not exist, nothing to delete
	}

	baseFolder := path.Join(v.Realm.String(), DataFolder)
	storeDir := path.Join(baseFolder, getSegmentDir(v.Config.SegmentInterval))
	storeName := generateFilename(now)

	_, err = v.writeRecord(name, "", PendingWrite|Deleted, nil)
	if err != nil {
		return core.Error(core.FileError, "cannot write record for file %s in %s", name, v.Realm, err)
	}

	head, err := encodeHead(v.Realm, file, v.UserID, v.getKey)
	if err != nil {
		return core.Error(core.DbError, "cannot encode head in Bao.Delete", err)
	}

	v.scheduleChangeFile()
	defer v.completeChangeFile()
	err = store.WriteFile(v.store, path.Join(storeDir, "h", storeName), head)
	if err != nil {
		return core.Error(core.FileError, "cannot write head for file %s in %s", name, file.Realm, err)
	}

	switch {
	case options&AsyncOperation != 0:
		go v.wipe(file)
	case options&ScheduledOperation != 0:
		// Do nothing, wipe will be handled later
	default:
		err = v.wipe(file)
		if err != nil {
			return core.Error(core.FileError, "cannot wipe file %s in %s", name, file.Realm, err)
		}
	}
	core.End("")
	return nil
}

func (v *Vault) wipe(file File) error {
	core.Start("wiping file %s in %s", file.Name, file.Realm)

	ph := path.Join(file.StoreDir, "/b", file.StoreName)
	err := v.store.Delete(ph)
	if err != nil {
		return core.Error(core.DbError, "cannot delete file %s in %s", file.Name, file.Realm, err)
	}

	core.End("successfully wiped file %s in %s", file.Name, file.Realm)
	return nil
}
