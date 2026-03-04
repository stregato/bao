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

	baseFolder := v.dataRoot()
	storeDir := path.Join(baseFolder, getSegmentDir(v.Config.SegmentInterval))
	storeName := generateFilename(now)

	tombstone := file
	tombstone.Name = nameWithoutEncryptionToken(name)
	tombstone.LocalCopy = ""
	tombstone.StoreDir = storeDir
	tombstone.StoreName = storeName
	tombstone.ModTime = now
	retention := effectiveRetention(v.Config.Retention, options.Retention)
	tombstone.ExpiresAt = truncateToSecond(now.Add(retention))
	tombstone.Flags |= PendingWrite | Deleted
	tombstone.Flags &^= PendingRead

	tombstone, err = v.writeFileHeadToDB(tombstone)
	if err != nil {
		return core.Error(core.FileError, "cannot write record for file %s", name, err)
	}

	encMethod, ecRecipient, err := v.encryptionMethodForFile(tombstone)
	if err != nil {
		return core.Error(core.ParseError, "cannot determine encryption mode for %s", name, err)
	}
	head, err := encodeHead(encMethod, tombstone, ecRecipient, v.UserSecret, v.getKey)
	if err != nil {
		return core.Error(core.DbError, "cannot encode head in Bao.Delete", err)
	}

	v.scheduleChangeFile()
	defer v.completeChangeFile()
	err = store.WriteFile(v.store, path.Join(storeDir, "h", storeName), head)
	if err != nil {
		return core.Error(core.FileError, "cannot write head for file %s", name, err)
	}

	switch {
	case options.Async:
		go v.wipe(file)
	case options.Scheduled:
		// Do nothing, wipe will be handled later
	default:
		err = v.wipe(file)
		if err != nil {
			return core.Error(core.FileError, "cannot wipe file %s", name, err)
		}
	}
	core.End("")
	return nil
}

func (v *Vault) wipe(file File) error {
	core.Start("wiping file %s", file.Name)

	ph := path.Join(file.StoreDir, "/b", file.StoreName)
	err := v.store.Delete(ph)
	if err != nil {
		return core.Error(core.DbError, "cannot delete file %s", file.Name, err)
	}

	core.End("successfully wiped file %s", file.Name)
	return nil
}
