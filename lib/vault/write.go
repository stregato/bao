package vault

import (
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/store"
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

func (v *Vault) writeRecord(dest, source string, flags Flags, attrs []byte) (File, error) {
	core.Start("writing record to %s", dest)
	now := core.Now()

	var size int64
	if source != "" {
		stat, err := os.Stat(source)
		if err != nil {
			return File{}, core.Error(core.FileError, "cannot stat source file %s in Bao.Write, name %v, source %v", dest, dest, source, err)
		}
		if stat.IsDir() {
			return File{}, core.Error(core.FileError, "source file %s is a directory in Bao.Write, name %v, source %v", dest, dest, source, err)
		}
		size = stat.Size()
	}
	if v.Config.MaxStorage > 0 && v.allocatedSize+size > v.Config.MaxStorage {
		return File{}, core.Error(core.FileError, "cannot write file %s in vaultgroup %s: allocated size limit exceeded", dest, v.ID, os.ErrPermission)
	}

	baseFolder := path.Join(v.Realm.String(), DataFolder)

	var keyId uint64
	var err error

	switch v.Realm {
	case All:
	case Home:
	default:
		keyId, _, err = v.getLastKeyFromDB()
		if err != nil {
			return File{}, core.Error(core.DbError, "cannot get key id for %v", dest, err)
		}
	}

	file := File{
		Name:          dest,                                                           // Name of the file
		Realm:         v.Realm,                                                        // Realm to which the file belongs
		Size:          size,                                                           // Size of the file
		AllocatedSize: 0,                                                              // Allocated size, to be updated later
		ModTime:       now,                                                            // Use current time as modification time
		IsDir:         false,                                                          // Not a directory
		Flags:         flags,                                                          // Flags for the file, e.g., Pending, Deleted
		Attrs:         attrs,                                                          // Optional attributes
		LocalCopy:     source,                                                         // Local path of the file
		StoreDir:      path.Join(baseFolder, getSegmentDir(v.Config.SegmentInterval)), // Directory in the store.where the file is located
		StoreName:     generateFilename(now),                                          // Name of the file in the storage
		AuthorId:      v.UserSecret.PublicIDMust(),                                    // Author ID
		KeyId:         keyId,                                                          // Key ID for encryption
	}

	file, err = v.writeFileHeadToDB(file)
	if err != nil {
		return File{}, core.Error(core.DbError, "cannot write file head to DB for %s", dest, err)
	}

	core.End("successfully wrote record to %s", dest)
	return file, nil
}

func (v *Vault) writeFile(file File, progress chan int64) error {
	core.Start("file %s", file.Name)
	now := core.Now()

	v.ioThrottleCh <- struct{}{} // Throttle the I/O operations
	defer func() {
		<-v.ioThrottleCh      // Release the throttle after the operation
		v.completeIo(file.Id) // Mark the I/O operation as complete
		if progress != nil {
			close(progress) // Close the progress channel if it was provided
		}
	}()

	file.Flags &= ^PendingWrite // Clear the PendingWrite flag
	head, err := encodeHead(v.Realm, file, v.UserSecret, v.getKey)
	if err != nil {
		return core.Error(core.EncodeError, "cannot encode head in Bao.Write", err)
	}

	var err2 error
	var wg sync.WaitGroup
	wg.Add(1)

	v.scheduleChangeFile() // Schedule the change file operation
	defer v.completeChangeFile()

	go func() {
		storePath := path.Join(file.StoreDir, "h", file.StoreName)
		core.Start("path %s", storePath)
		err := store.WriteFile(v.store, storePath, head)
		if err != nil {
			err2 = core.Error(core.FileError, "cannot write head for file %s in Bao.Write, name %v, group %v",
				file.Name, file.StoreName, file.StoreDir, err)
		} else {
			v.notifyChange(path.Join(file.StoreDir, file.StoreName))
			core.End("")
		}
		wg.Done()
	}()
	if file.LocalCopy != "" {
		f, err := os.Open(file.LocalCopy)
		if err != nil {
			return core.Error(core.FileError, "cannot open local file %s in Bao.Write, name %v, group %v",
				file.LocalCopy, file.Name, file.StoreDir, err)
		}
		defer f.Close()

		r, err := encryptReader(v.Realm, file, f, v.getKey)
		if err != nil {
			return core.Error(core.FileError, "cannot encrypt reader for file %s in Bao.Write, name %v, group %v",
				file.Name, file.LocalCopy, file.StoreDir, err)
		}
		storePath := path.Join(file.StoreDir, "b", file.StoreName)
		err = v.store.Write(storePath, r, progress)
		if err != nil {
			return core.Error(core.FileError, "cannot write body for file %s in Bao.Write, name %v, group %v",
				file.Name, file.LocalCopy, file.StoreDir, err)
		}
	}
	wg.Wait()        // Wait for the head to be written
	if err2 != nil { // if writing the head failed in the goroutine
		return err2
	}

	v.UpdateFileAllocatedSize(file.Id, file.Size+int64(len(head)))
	v.UpdateFileFlags(file.Id, file.Flags) // Update the file flags in the database
	v.allocatedSize += file.Size + int64(len(head))

	core.End("elapsed %s", core.Since(now))
	return nil

}

// Write writes a file to the vault.
// It creates a record for the file in the database and writes the file to the store.
// If the Sync option is set, it writes the file synchronously, otherwise it writes it asynchronously.
// If the Scheduled option is set, it schedules the file to be written later and the parameter `progress` is ignored.
func (v *Vault) Write(dest, source string, attrs []byte, options IOOption, progress chan int64) (File, error) {
	core.Start("dest %s, source %s, options %d", dest, source, options)
	now := core.Now()

	file, err := v.writeRecord(dest, source, PendingWrite, attrs)
	if err != nil {
		return File{}, core.Error(core.FileError, "cannot write record for file %s in %s", dest, v.Realm, err)
	}

	switch {
	case options&AsyncOperation != 0: // If Async is set, we can write the file asynchronously
		v.scheduleIo(file.Id) // Schedule the IO operation
		go v.writeFile(file, progress)

	case options&ScheduledOperation != 0: // If Scheduled is set, let use the scheduler to write the file later
	default:
		// If neither Async nor Scheduled is set, we can write the file synchronously
		err = v.writeFile(file, progress)
		if err != nil {
			return File{}, core.Error(core.FileError, "cannot write file %s", file.Name, err)
		}
	}

	core.End("elapsed %s", core.Since(now))
	return file, nil
}

func (v *Vault) scheduleChangeFile() {
	core.Start("scheduling change file")

	v.ioWritingWg.Add(1)
	if !atomic.CompareAndSwapInt32(&v.ioLastChangeRunning, 0, 1) {
		return // already running
	}
	time.AfterFunc(time.Second, func() {
		v.ioWritingWg.Wait()
		store.WriteFile(v.store, path.Join(v.Realm.String(), DataFolder, ".change"), []byte{})
		defer atomic.StoreInt32(&v.ioLastChangeRunning, 0)
	})
	core.End("scheduled change file")
}

func (v *Vault) completeChangeFile() {
	core.Start("completing change file")
	v.ioWritingWg.Done()
	core.End("completed change file")
}
