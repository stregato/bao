package vault

import (
	"os"
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

func (v *Vault) readFile(file File, progress chan int64) error {
	core.Start("reading file %s", file.Name)
	now := time.Now()
	v.ioThrottleCh <- struct{}{}
	defer func() {
		<-v.ioThrottleCh
		v.completeIo(file.Id) // Mark the I/O operation as complete
		if progress != nil {
			close(progress)
		}
	}()

	f, err := os.Create(file.LocalCopy)
	if err != nil {
		return core.Errorw("cannot create file %s", file.LocalCopy, err)
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(file.LocalCopy) // Remove the file if there was an error
		}
	}()

	writer, err := decryptWriter(v.Realm, v.UserID, file, f, v.getKey)
	if err != nil {
		return core.Errorw("cannot create decrypt writer for %s", file.Name, err)
	}

	//	bodyDir := strings.ReplaceAll(file.StoreDir, "/h", "/b")
	err = v.store.Read(path.Join(file.StoreDir, "/b", file.StoreName), nil, writer, progress)
	if err != nil {
		return core.Errorw("cannot read file %s", file.Name, err)
	}

	if file.Flags&PendingRead != 0 {
		file.Flags &^= PendingRead // Clear the PendingRead flag
		err = v.UpdateFileFlags(file.Id, file.Flags)
		if err != nil {
			return core.Errorw("cannot clear PendingRead flag for file %s", file.Name, err)
		}
	}

	core.End("successfully read file %s in %s", file.Name, time.Since(now))
	return nil
}

func (v *Vault) UpdateFileAllocatedSize(id FileId, allocatedSize int64) error {
	core.Start("updating allocated size for file with id %d to %d", id, allocatedSize)
	_, err := v.DB.Exec("UPDATE_FILE_ALLOCATED_SIZE", sqlx.Args{"vault": v.ID, "id": id, "allocatedSize": allocatedSize})
	if err != nil {
		return core.Errorw("cannot update file allocated size for id %d", id, err)
	}
	core.End("updated allocated size for file with id %d to %d", id, allocatedSize)
	return nil
}

func (v *Vault) UpdateFileFlags(id FileId, flags Flags) error {
	core.Start("updating flags for file with id %d to %d", id, flags)
	_, err := v.DB.Exec("UPDATE_FILE_FLAGS", sqlx.Args{"vault": v.ID, "id": id, "flags": flags})
	if err != nil {
		return core.Errorw("cannot update file flags for id %d", id, err)
	}
	core.End("updated flags for file with id %d to %d", id, flags)
	return nil
}

func (v *Vault) Read(name string, dest string, options IOOption, progress chan int64) (File, error) {
	core.Start("reading file %s to %s", name, dest)
	now := time.Now()
	file, found, err := v.queryFileByName(name)
	if err != nil {
		return File{}, core.Errorw("cannot query file %s", name, err)
	}
	if !found {
		return File{}, os.ErrNotExist
	}

	file.Flags |= PendingRead                    // Ensure the PendingRead flag is set
	err = v.UpdateFileFlags(file.Id, file.Flags) // Set the PendingRead flag
	if err != nil {
		return File{}, core.Errorw("cannot set flags for file %s", name, err)
	}
	file.LocalCopy = dest                      // Set the local name for the file
	err = v.updateFileLocalName(file.Id, dest) // Update the local name in the database
	if err != nil {
		return File{}, core.Errorw("cannot update local name for file %s", name, err)
	}

	switch {
	case options&AsyncOperation != 0: // Asynchronous operation, set the file flags to PendingRead and schedule the read operation
		err = v.UpdateFileFlags(file.Id, file.Flags)
		if err != nil {
			return File{}, core.Errorw("cannot set flags for file %s", name, err)
		}
		v.scheduleIo(file.Id) // Schedule the I/O operation
		go v.readFile(file, progress)
		core.Info("file %s will be read asynchronously", name)
	case options&ScheduledOperation != 0: // Scheduled operation, set the file flags to PendingRead
		err = v.UpdateFileFlags(file.Id, file.Flags)
		if err != nil {
			return File{}, core.Errorw("cannot set flags for file %s", name, err)
		}
		core.Info("file %s is scheduled for reading", name)
	default: // Default case, read the file synchronously
		err := v.readFile(file, progress)
		if err != nil {
			return File{}, core.Errorw("cannot read file %s", name, err)
		}
		core.Info("successfully read file %s to %s in %s", name, dest, time.Since(now))
	}

	core.End("")
	return file, nil
}
