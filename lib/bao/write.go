package bao

import (
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/storage"
	"golang.org/x/crypto/blake2b"
)

func getIv(name string) ([]byte, error) {
	core.Start("getting IV for name %s", name)
	name = path.Base(name)
	colonIdx := strings.LastIndex(name, ":")
	if colonIdx != -1 {
		name = name[:colonIdx]
	}

	hash, err := blake2b.New256(nil)
	if err != nil {
		return nil, err
	}
	hash.Write([]byte(name))
	core.End("successfully retrieved IV for name %s", name)
	return hash.Sum(nil)[:16], nil
}

func (s *Bao) writeRecord(dest, source string, group Group, flags Flags, attrs []byte) (File, error) {
	core.Start("writing record to %s", dest)
	now := core.Now()

	var size int64
	if source != "" {
		stat, err := os.Stat(source)
		if err != nil {
			return File{}, core.Errorw("cannot stat source file %s in Bao.Write, name %v, group %v", dest, group, source, err)
		}
		if stat.IsDir() {
			return File{}, core.Errorw("source file %s is a directory in Bao.Write, name %v, group %v", dest, group, source, err)
		}
		size = stat.Size()
	}
	if s.Config.MaxStorage > 0 && s.allocatedSize+size > s.Config.MaxStorage {
		return File{}, core.Errorw("cannot write file %s in group %s: allocated size limit exceeded", dest, group, os.ErrPermission)
	}

	baseFolder := path.Join(DataFolder, string(group))

	keyId, _, err := s.getLastKeyFromDB(group)
	if err != nil {
		return File{}, core.Errorw("cannot get key id for group %s in Bao.Write, name %v, group %v", group, dest, group, err)
	}

	file := File{
		Name:          dest,                                                           // Name of the file
		Group:         group,                                                          // Group to which the file belongs
		Size:          size,                                                           // Size of the file
		AllocatedSize: 0,                                                              // Allocated size, to be updated later
		ModTime:       now,                                                            // Use current time as modification time
		IsDir:         false,                                                          // Not a directory
		Flags:         flags,                                                          // Flags for the file, e.g., Pending, Deleted
		Attrs:         attrs,                                                          // Optional attributes
		LocalCopy:     source,                                                         // Local path of the file
		StoreDir:      path.Join(baseFolder, getSegmentDir(s.Config.SegmentInterval)), // Directory in the storage where the file is located
		StoreName:     generateFilename(now),                                          // Name of the file in the storage
		AuthorId:      s.UserId.PublicIDMust(),                                        // Author ID
		KeyId:         keyId,                                                          // Key ID for encryption
	}

	file, err = s.writeFileHeadToDB(file)
	if err != nil {
		return File{}, core.Errorw("cannot write file head to DB for %s", dest, err)
	}

	core.End("successfully wrote record to %s", dest)
	return file, nil
}

func (s *Bao) writeFile(file File, progress chan int64) error {
	core.Start("file %s", file.Name)
	now := core.Now()

	if s.store == nil {
		store, err := storage.Open(s.Url)
		if err != nil {
			return core.Errorw("cannot open store %s in Bao.Write", s.Url, err)
		}
		s.store = store
	}

	s.ioThrottleCh <- struct{}{} // Throttle the I/O operations
	defer func() {
		<-s.ioThrottleCh      // Release the throttle after the operation
		s.completeIo(file.Id) // Mark the I/O operation as complete
		if progress != nil {
			close(progress) // Close the progress channel if it was provided
		}
	}()

	if s.store == nil {
		store, err := storage.Open(s.Url)
		if err != nil {
			return core.Errorw("cannot open store %s in Bao.Write", s.Url, err)
		}
		s.store = store
	}

	file.Flags &= ^PendingWrite // Clear the PendingWrite flag
	head, err := encodeHead(file, s.UserId, s.getKey, s.getGroupFromKey)
	if err != nil {
		return core.Errorw("cannot encode head in Bao.Write", err)
	}

	var err2 error
	var wg sync.WaitGroup
	wg.Add(1)

	s.scheduleChangeFile(file.Group) // Schedule the change file operation
	defer s.completeChangeFile(file.Group)

	go func() {
		storagePath := path.Join(file.StoreDir, "h", file.StoreName)
		core.Start("path %s", storagePath)
		err := storage.WriteFile(s.store, storagePath, head)
		if err != nil {
			err2 = core.Errorw("cannot write head for file %s in Bao.Write, name %v, group %v",
				file.Name, file.StoreName, file.StoreDir, err)
		} else {
			core.End("")
		}
		wg.Done()
	}()
	if file.LocalCopy != "" {
		f, err := os.Open(file.LocalCopy)
		if err != nil {
			return core.Errorw("cannot open local file %s in Bao.Write, name %v, group %v",
				file.LocalCopy, file.Name, file.StoreDir, err)
		}
		defer f.Close()

		r, err := encryptReader(file, f, s.getKey, s.getGroupFromKey)
		if err != nil {
			return core.Errorw("cannot encrypt reader for file %s in Bao.Write, name %v, group %v",
				file.Name, file.LocalCopy, file.StoreDir, err)
		}

		storagePath := path.Join(file.StoreDir, "b", file.StoreName)
		err = s.store.Write(storagePath, r, progress)
		if err != nil {
			return core.Errorw("cannot write body for file %s in Bao.Write, name %v, group %v",
				file.Name, file.LocalCopy, file.StoreDir, err)
		}
	}
	wg.Wait()        // Wait for the head to be written
	if err2 != nil { // if writing the head failed in the goroutine
		return err2
	}

	s.UpdateFileAllocatedSize(file.Id, file.Size+int64(len(head)))
	s.UpdateFileFlags(file.Id, file.Flags) // Update the file flags in the database
	s.allocatedSize += file.Size + int64(len(head))

	core.End("elapsed %s", core.Since(now))
	return nil

}

// Write writes a file to the stash.
// It creates a record for the file in the database and writes the file to the storage.
// If the Sync option is set, it writes the file synchronously, otherwise it writes it asynchronously.
// If the Scheduled option is set, it schedules the file to be written later and the parameter `progress` is ignored.
func (s *Bao) Write(dest, source string, group Group, attrs []byte, options IOOption, progress chan int64) (File, error) {
	core.Start("dest %s, source %s, group %s, options %d", dest, source, group, options)
	now := core.Now()
	if strings.HasPrefix(string(group), "#") {
		return File{}, core.Errorw("group %s is a special group and cannot be used for writing files", group, nil)
	}

	file, err := s.writeRecord(dest, source, group, PendingWrite, attrs)
	if err != nil {
		return File{}, core.Errorw("cannot write record for file %s in %s", dest, group, err)
	}

	switch {
	case options&AsyncOperation != 0: // If Async is set, we can write the file asynchronously
		s.scheduleIo(file.Id) // Schedule the IO operation
		go s.writeFile(file, progress)

	case options&ScheduledOperation != 0: // If Scheduled is set, let use the scheduler to write the file later
	default:
		// If neither Async nor Scheduled is set, we can write the file synchronously
		err = s.writeFile(file, progress)
		if err != nil {
			return File{}, core.Errorw("cannot write file %s", file.Name, err)
		}
	}

	core.End("elapsed %s", core.Since(now))
	return file, nil
}

func (s *Bao) scheduleChangeFile(group Group) {
	core.Start("scheduling change file for group %s", group)
	wg := s.ioWritingWgMaps[group]
	if wg == nil {
		wg = &sync.WaitGroup{}
		s.ioWritingWgMaps[group] = wg
	}

	wg.Add(1)
	if !atomic.CompareAndSwapInt32(&s.ioLastChangeRunning, 0, 1) {
		return // already running
	}
	time.AfterFunc(time.Second, func() {
		wg.Wait()
		storage.WriteFile(s.store, path.Join(DataFolder, group.String(), ".last_change"), []byte{})
		defer atomic.StoreInt32(&s.ioLastChangeRunning, 0)
	})
	core.End("scheduled change file for group %s", group)
}

func (s *Bao) completeChangeFile(group Group) {
	core.Start("completing change file for group %s", group)
	wg := s.ioWritingWgMaps[group]
	if wg != nil {
		wg.Done()
	}
	core.End("completed change file for group %s", group)
}

func (s *Bao) checkAndUpdateExternalChange(group Group) bool {
	core.Start("group %s", group)

	_, tm, _, _, err := s.DB.GetSetting(path.Join(s.Id, "last_change", group.String()))
	if err != nil {
		core.End("no changes")
		return true // If we cannot get the last change, we assume there are changes
	}

	lastChangeFile := path.Join(DataFolder, group.String(), ".last_change")
	stat, err := s.store.Stat(lastChangeFile)
	if os.IsNotExist(err) {
		core.End("no change file %s", lastChangeFile)
		return false
	}
	if err != nil {
		core.Errorw("FUNC_END[checkAndUpdateExternalChange]: cannot stat last change file %s", lastChangeFile, err)
		core.End("no changes")
		return false
	}
	s.DB.SetSetting(path.Join(s.Id, "last_change", group.String()), "", stat.ModTime().Unix(), 0, nil)

	if stat.ModTime().Unix() > tm {
		core.End("change file %s newer -> has changed", lastChangeFile)
		return true
	} else {
		core.End("change file %s older or equal -> not changed", lastChangeFile)
		return false
	}
}
