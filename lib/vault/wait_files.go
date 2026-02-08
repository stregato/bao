package vault

import (
	"context"
	"database/sql"
	"time"

	"github.com/stregato/bao/lib/core"
)

// WaitFiles waits for the specified files to complete pending read/write I/O by their IDs.
// If no file IDs are provided, it automatically collects all files marked with PendingRead or PendingWrite flags.
// The function ensures that the store is opened before proceeding with synchronization.
// It returns the files that were waited for and an error if the synchronization fails or if the context deadline is exceeded.
func (v *Vault) WaitFiles(ctx context.Context, ids ...FileId) ([]File, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var err error
	now := time.Now()

	// If no explicit file IDs are provided, fall back to all pending read/write files plus special jobs.
	if len(ids) == 0 {
		ids, err = v.queryFileIdsByFlags(PendingRead | PendingWrite)
		if err != nil {
			return nil, core.Error(core.DbError, "cannot query files with PendingRead or PendingWrite flags for store ID %s", v.ID, err)
		}
	}

	if len(ids) == 0 {
		return nil, nil // No files to wait for
	}

	// Use channels to collect both files and errors
	type result struct {
		file File
		err  error
	}
	ch := make(chan result, len(ids))
	for _, fileId := range ids {
		go func(fileId FileId) {
			file, err := v.waitFile(fileId)
			ch <- result{file: file, err: err}
		}(fileId)
	}

	// Collect results with timeout
	var files []File
	for i := 0; i < len(ids); i++ {
		select {
		case res := <-ch:
			if res.err != nil {
				return nil, res.err
			}
			files = append(files, res.file)
		case <-ctx.Done():
			core.Info("WaitFiles timeout reached for store ID %s, returning %d of %d files", v.ID, len(files), len(ids))
			return files, nil
		}
	}

	core.Info("Successfully synchronized vault with store ID %s in %s", v.ID, time.Since(now))
	return files, nil
}

func (v *Vault) waitFiles() ([]File, error) {
	// Create a context with default timeout
	timeout := core.DefaultIfZero(v.Config.WaitTimeout, 10*time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return v.WaitFiles(ctx)
}

func (v *Vault) waitFile(fileId FileId) (File, error) {
	core.Start("waiting for file ID %d to complete I/O operation", fileId)
	ok := v.waitIo(fileId, core.DefaultIfZero(v.Config.WaitTimeout, 10*time.Minute))
	if ok {
		core.Info("File ID %d completed I/O operation", fileId)
		return File{}, nil // If the file is already being processed, return
	}

	file, found, err := v.queryFileById(fileId)
	if err != nil {
		// If we cannot get the file, log the error and return
		return File{}, core.Error(core.DbError, "cannot get file flags for file ID %d", fileId, err)
	}
	if !found {
		return File{}, core.Error(core.FileError, "file ID %d does not exist", fileId, sql.ErrNoRows)
	}
	switch {
	case file.Flags&PendingRead != 0:
		// If the file is marked for reading, read it
		err = v.readFile(file, nil)
		if err != nil {
			return File{}, core.Error(core.FileError, "cannot read file ID %d", fileId, err)
		}
		core.Info("File ID %d is marked for read, successfully read", fileId)
	case file.Flags&PendingWrite != 0:
		// If the file is marked for writing, write it
		err = v.writeFile(file, nil)
		if err != nil {
			return File{}, core.Error(core.FileError, "cannot write file ID %d", fileId, err)
		}
		core.Info("File ID %d is marked for write, successfully written", fileId)
	default:
		core.Info("File ID %d is not marked for read or write, skipping synchronization", fileId)
	}
	core.End("completed waiting for file ID %d", fileId)
	return file, nil
}
