package bao

import (
	"path"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/storage"
)

func (s *Bao) Delete(name string, options IOOption) error {
	core.Start("name %s", name)
	now := core.Now()

	file, found, err := s.queryFileByName(name)
	if err != nil {
		return core.Errorw("cannot query file %s", name, err)
	}
	if !found {
		return nil // File does not exist, nothing to delete
	}

	baseFolder := path.Join(DataFolder, file.Group.String())
	storageDir := path.Join(baseFolder, getSegmentDir(s.Config.SegmentInterval))
	storageName := generateFilename(now)

	_, err = s.writeRecord(name, "", file.Group, PendingWrite|Deleted, nil)
	if err != nil {
		return core.Errorw("cannot write record for file %s in %s", name, file.Group, err)
	}

	head, err := encodeHead(file, s.UserId, s.getKey, s.getGroupFromKey)
	if err != nil {
		return core.Errorw("cannot encode head in Bao.Delete", err)
	}

	s.scheduleChangeFile(file.Group)
	defer s.completeChangeFile(file.Group)
	err = storage.WriteFile(s.store, path.Join(storageDir, "h", storageName), head)
	if err != nil {
		return core.Errorw("cannot write head for file %s in %s", name, file.Group, err)
	}

	switch {
	case options&AsyncOperation != 0:
		go s.wipe(file)
	case options&ScheduledOperation != 0:
		// Do nothing, wipe will be handled later
	default:
		err = s.wipe(file)
		if err != nil {
			return core.Errorw("cannot wipe file %s in %s", name, file.Group, err)
		}
	}
	core.End("")
	return nil
}

func (s *Bao) wipe(file File) error {
	core.Start("wiping file %s in %s", file.Name, file.Group)

	ph := path.Join(file.StoreDir, "/b", file.StoreName)
	err := s.store.Delete(ph)
	if err != nil {
		return core.Errorw("cannot delete file %s in %s", file.Name, file.Group, err)
	}

	core.End("successfully wiped file %s in %s", file.Name, file.Group)
	return nil
}
