package vault

import (
	"database/sql"
	"sync"
	"time"

	"github.com/stregato/bao/lib/core"
)

// WaitFiles waits for the specified files to complete pending read/write I/O by their IDs.
// If no file IDs are provided, it automatically collects all files marked with PendingRead or PendingWrite flags,
// The function ensures that the store is opened before proceeding with synchronization.
// It returns an error if the synchronization fails.
func (v *Vault) WaitFiles(ids ...FileId) error {
	var err error
	now := time.Now()

	// If no explicit file IDs are provided, fall back to all pending read/write files plus special jobs.
	// (Previous behavior returned an error, which prevented callers from forcing a flush of pending writes.)
	if len(ids) == 0 {
		ids, err = v.queryFileIdsByFlags(PendingRead | PendingWrite)
		if err != nil {
			return core.Errorw("cannot query files with PendingRead or PendingWrite flags for store ID %s", v.ID, err)
		}
	}
	// ids already populated (either provided by caller or auto-collected above)

	var wg sync.WaitGroup

	for _, fileId := range ids {
		wg.Add(1)
		go v.waitFile(fileId, &wg) // Use a goroutine to handle each file
	}
	wg.Wait()
	core.Info("Successfully synchronized vault with store ID %s in %s", v.ID, time.Since(now))
	return nil
}

func (v *Vault) waitFiles() error {
	ids, err := v.queryFileIdsByFlags(PendingRead | PendingWrite)
	if err != nil {
		return core.Errorw("cannot query files with PendingRead or PendingWrite flags for store ID %s", v.ID, err)
	}

	var wg sync.WaitGroup
	for _, fileId := range ids {
		wg.Add(1)
		go v.waitFile(fileId, &wg) // Use a goroutine to handle each file
	}
	wg.Wait()
	return nil
}

func (v *Vault) waitFile(fileId FileId, wg *sync.WaitGroup) error {
	core.Start("waiting for file ID %d to complete I/O operation", fileId)
	defer wg.Done()
	ok := v.waitIo(fileId, core.DefaultIfZero(v.Config.WaitTimeout, 10*time.Minute))
	if ok {
		core.Info("File ID %d completed I/O operation", fileId)
		return nil // If the file is already being processed, return
	}

	file, found, err := v.queryFileById(fileId)
	if err != nil {
		// If we cannot get the file, log the error and return
		return core.Errorw("cannot get file flags for file ID %d", fileId, err)
	}
	if !found {
		return core.Errorw("file ID %d does not exist", fileId, sql.ErrNoRows)
	}
	switch {
	case file.Flags&PendingRead != 0:
		// If the file is marked for reading, read it
		err = v.readFile(file, nil)
		if err != nil {
			return core.Errorw("cannot read file ID %d", fileId, err)
		}
		core.Info("File ID %d is marked for read, successfully read", fileId)
	case file.Flags&PendingWrite != 0:
		// If the file is marked for writing, write it
		err = v.writeFile(file, nil)
		if err != nil {
			return core.Errorw("cannot write file ID %d", fileId, err)
		}
		core.Info("File ID %d is marked for write, successfully written", fileId)
	default:
		core.Info("File ID %d is not marked for read or write, skipping synchronization", fileId)
	}
	core.End("completed waiting for file ID %d", fileId)
	return nil
}
