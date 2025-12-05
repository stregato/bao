package bao

import (
	"io"
	"os"
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

func (s *Bao) readFile(file File, progress chan int64) error {
	core.Start("reading file %s", file.Name)
	now := time.Now()
	s.ioThrottleCh <- struct{}{}
	defer func() {
		<-s.ioThrottleCh
		s.completeIo(file.Id) // Mark the I/O operation as complete
		if progress != nil {
			close(progress)
		}
	}()

	iv, err := getIv(file.Name)
	if err != nil {
		return err
	}
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

	var w io.Writer
	switch {
	case file.KeyId == 0:
		w = f // No encryption, write directly to the file
	case file.KeyId&(1<<63) != 0: // EC encryption
		w, err = security.EcDecryptWriter(s.UserId, f, iv)
		if err != nil {
			return core.Errorw("cannot create ec decrypt writer for %s", file.Name, err)
		}
	default: // AES encryption
		key, err := s.getKey(file.KeyId)
		if err != nil {
			return core.Errorw("cannot get key for file %s", file.Name, err)
		}

		w, err = security.DecryptWriter(f, key, iv)
		if err != nil {
			return core.Errorw("cannot create decrypt writer for %s", file.Name, err)
		}
	}

	//	bodyDir := strings.ReplaceAll(file.StoreDir, "/h", "/b")
	err = s.store.Read(path.Join(file.StoreDir, "/b", file.StoreName), nil, w, progress)
	if err != nil {
		return core.Errorw("cannot read file %s", file.Name, err)
	}

	if file.Flags&PendingRead != 0 {
		file.Flags &^= PendingRead // Clear the PendingRead flag
		err = s.UpdateFileFlags(file.Id, file.Flags)
		if err != nil {
			return core.Errorw("cannot clear PendingRead flag for file %s", file.Name, err)
		}
	}

	core.End("successfully read file %s in %s", file.Name, time.Since(now))
	return nil
}

func (s *Bao) UpdateFileAllocatedSize(id FileId, allocatedSize int64) error {
	core.Start("updating allocated size for file with id %d to %d", id, allocatedSize)
	_, err := s.DB.Exec("UPDATE_FILE_ALLOCATED_SIZE", sqlx.Args{"store": s.Id, "id": id, "allocatedSize": allocatedSize})
	if err != nil {
		return core.Errorw("cannot update file allocated size for id %d", id, err)
	}
	core.End("updated allocated size for file with id %d to %d", id, allocatedSize)
	return nil
}

func (s *Bao) UpdateFileFlags(id FileId, flags Flags) error {
	core.Start("updating flags for file with id %d to %d", id, flags)
	_, err := s.DB.Exec("UPDATE_FILE_FLAGS", sqlx.Args{"store": s.Id, "id": id, "flags": flags})
	if err != nil {
		return core.Errorw("cannot update file flags for id %d", id, err)
	}
	core.End("updated flags for file with id %d to %d", id, flags)
	return nil
}

func (s *Bao) Read(name string, dest string, options IOOption, progress chan int64) (File, error) {
	core.Start("reading file %s to %s", name, dest)
	now := time.Now()
	file, found, err := s.queryFileByName(name)
	if err != nil {
		return File{}, core.Errorw("cannot query file %s", name, err)
	}
	if !found {
		return File{}, os.ErrNotExist
	}

	file.Flags |= PendingRead                    // Ensure the PendingRead flag is set
	err = s.UpdateFileFlags(file.Id, file.Flags) // Set the PendingRead flag
	if err != nil {
		return File{}, core.Errorw("cannot set flags for file %s", name, err)
	}
	file.LocalCopy = dest                      // Set the local name for the file
	err = s.updateFileLocalName(file.Id, dest) // Update the local name in the database
	if err != nil {
		return File{}, core.Errorw("cannot update local name for file %s", name, err)
	}

	switch {
	case options&AsyncOperation != 0: // Asynchronous operation, set the file flags to PendingRead and schedule the read operation
		err = s.UpdateFileFlags(file.Id, file.Flags)
		if err != nil {
			return File{}, core.Errorw("cannot set flags for file %s", name, err)
		}
		s.scheduleIo(file.Id) // Schedule the I/O operation
		go s.readFile(file, progress)
		core.Info("file %s will be read asynchronously", name)
	case options&ScheduledOperation != 0: // Scheduled operation, set the file flags to PendingRead
		err = s.UpdateFileFlags(file.Id, file.Flags)
		if err != nil {
			return File{}, core.Errorw("cannot set flags for file %s", name, err)
		}
		core.Info("file %s is scheduled for reading", name)
	default: // Default case, read the file synchronously
		err := s.readFile(file, progress)
		if err != nil {
			return File{}, core.Errorw("cannot read file %s", name, err)
		}
		core.Info("successfully read file %s to %s in %s", name, dest, time.Since(now))
	}

	core.End("")
	return file, nil
}
