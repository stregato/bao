package vault

import (
	"database/sql"
	"path"
	"sync"
	"time"

	"sort"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/store"
)

// Sync synchronizes the filesystem for the specified groups.
// If no groups are specified, it returns an error.
// It returns a list of new files that were added during the synchronization.
// The function ensures that the store is opened before proceeding with synchronization.
func (v *Vault) Sync() (newFiles []File, err error) {
	core.Start("synchronizing vault %s", v.ID)

	now := time.Now()

	baseDir := path.Join(string(v.Realm), DataFolder)
	hasChanged, err := v.hasChanged(baseDir)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot determine if vault has changed", err)
	}
	if !hasChanged {
		core.End("no changes detected in vault %s", v.ID)
		return nil, nil
	}

	// 1. Get the last store directory with prefix baseFolder
	lastStoreDir, err := v.findLastStoreDirIn(baseDir)
	if err != nil {
		return nil, core.Error(core.GenericError, "cannot find last store dir", err)
	}

	var minSegment string
	if lastStoreDir != "" {
		minSegment = path.Base(lastStoreDir)
	}
	maxSegment := getSegmentDir(v.Config.SegmentInterval)

	var segments []string
	if minSegment == maxSegment {
		segments = []string{minSegment} // If the latest batch is the same as the current, only use that
	} else {
		segments = v.listDirs(baseDir, minSegment, maxSegment)
	}

	var errX error
	for _, segment := range segments {
		storeDir := path.Join(baseDir, segment)
		ls, err2 := v.store.ReadDir(path.Join(storeDir, "h"), store.Filter{})
		if err2 != nil {
			continue // skip if directory does not exist or is not readable
		}

		knowns, err2 := v.getKnownFilesNames(storeDir)
		if err2 != nil {
			return nil, core.Error(core.DbError, "cannot get known files in sealed dir %s", storeDir, err2)
		}

		n := 0
		for _, v := range ls {
			if !knowns[v.Name()] {
				ls[n] = v
				n++
			}
		}
		ls = ls[:n] // Filter out known files

		parallelism := min(16, len(ls)) // Limit parallelism to the number of files
		in := make(chan string, parallelism)
		out := make(chan any, 10)
		var wait sync.WaitGroup

		for i := 0; i < parallelism; i++ {
			wait.Add(1)
			go func() { // Use a goroutine to handle each file
				defer wait.Done()
				name, ok := <-in
				if !ok {
					return // channel closed
				}

				file, err := v.syncronizeFile(storeDir, name)
				if err != nil {
					out <- err
				} else {
					out <- file
				}
			}()
		}
		go func() {
			for _, l := range ls {
				in <- l.Name() // send file name to the worker
			}
			close(in) // close the input channel after sending all file names
		}()
		go func() {
			wait.Wait() // wait for all workers to finish
			close(out)  // close the output channel
		}()

		for res := range out {
			switch v := res.(type) {
			case File:
				newFiles = append(newFiles, v)
			case error:
				if v != nil {
					errX = v
					core.Info("error synchronizing file in batch %s: %v", segment, v)
				}
			}
		}
	}
	if errX != nil {
		return nil, core.Error(core.GenericError, "errors occurred during synchronization", errX)
	}

	core.End("synchronized vault %s in %s, %d new files", v.ID, time.Since(now), len(newFiles))
	return newFiles, nil

}

func (v *Vault) findLastStoreDirIn(baseDir string) (string, error) {
	core.Start("baseDir %s", baseDir)

	var lastStoreDir string
	err := v.DB.QueryRow("GET_LAST_STORE_DIR", sqlx.Args{
		"vault":   v.ID,
		"baseDir": baseDir,
	}, &lastStoreDir)
	if err != nil && err != sql.ErrNoRows {
		return "", core.Error(core.DbError, "cannot get latest sealed file", err)
	}

	core.End("last store dir %s", lastStoreDir)
	return lastStoreDir, nil
}

func (v *Vault) listDirs(path string, min, max string) []string {
	core.Start("path %s, min %s, max %s", path, min, max)
	if path == max {
		core.End("path is already max")
		return []string{path}
	}
	entries, err := v.store.ReadDir(path, store.Filter{})
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if n >= min && n <= max {
			dirs = append(dirs, n)
		}
	}
	sort.Strings(dirs)
	core.End("found %d dirs in %s", len(dirs), path)
	return dirs
}

func (v *Vault) getKnownFilesNames(storeDir string) (map[string]bool, error) {
	core.Start("storeDir %s", storeDir)
	rows, err := v.DB.Query("GET_STORE_NAMES", sqlx.Args{"vault": v.ID, "storeDir": storeDir})
	if err == sql.ErrNoRows {
		core.End("no known files in store dir %s", storeDir)
		return nil, nil // No files in sealed directory
	}
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get files in sealed dir %s", storeDir, err)
	}
	defer rows.Close()
	files := make(map[string]bool)
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, core.Error(core.FileError, "cannot scan file name in sealed dir %s", storeDir, err)
		}
		files[name] = true
	}

	core.End("retrieved %d names in store dir %s", len(files), storeDir)
	return files, nil
}

func (v *Vault) syncronizeFile(storeDir, storeName string) (File, error) {
	core.Start("storeDir %s, storeName %s", storeDir, storeName)

	n := path.Join(storeDir, "h", storeName)
	head, err := store.ReadFile(v.store, n)
	if err != nil {
		return File{}, core.Error(core.FileError, "cannot read sealed file %s", n, err)
	}

	file, err := decodeHead(v.Realm, head, v.UserSecret, v.getKey, v.getUserByShortId)
	if err != nil {
		return File{}, core.Error(core.FileError, "cannot decode file head %s", n, err)
	}

	file.AllocatedSize = int64(len(head)) + file.Size
	file.StoreDir = storeDir
	file.StoreName = storeName
	file.Realm = v.Realm
	file, err = v.writeFileHeadToDB(file)
	if err != nil {
		return File{}, core.Error(core.DbError, "cannot write file head to DB for %s", n, err)
	}

	v.allocatedSize += file.AllocatedSize
	core.End("file %s, size %d, allocated %d, modTime %s", file.Name, file.Size, file.AllocatedSize, file.ModTime)
	return file, nil
}

func (v *Vault) writeFileHeadToDB(file File) (File, error) {
	core.Start("file %s", file.Name)
	dir, name := path.Split(file.Name)
	dir = path.Clean(dir)
	if file.AllocatedSize == 0 && file.Size > 0 {
		file.AllocatedSize = file.Size
	}

	r, err := v.DB.Exec("SET_FILE", sqlx.Args{
		"vault":          v.ID,
		"dir":            dir,
		"storeDir":       file.StoreDir,
		"storeName":      file.StoreName,
		"name":           name,
		"group":          file.Realm,
		"localCopy":      file.LocalCopy,
		"modTime":        file.ModTime.UnixMilli(),
		"size":           file.Size,
		"allocatedSize":  file.AllocatedSize,
		"flags":          file.Flags,
		"authorId":       file.AuthorId,
		"keyId":          file.KeyId,
		"encryptionType": 0,
		"attrs":          file.Attrs,
	})
	if err != nil {
		return File{}, core.Error(core.DbError, "cannot set file %s/%s", dir, name, err)
	}
	id, err := r.LastInsertId()
	if err != nil {
		return File{}, core.Error(core.DbError, "cannot get last insert id for file %s/%s", dir, name, err)
	}
	count, _ := r.RowsAffected()
	if dir == "" || count == 0 {
		core.End("file %s is a directory or already exists, skipping directory set", file.Name)
		return File{Id: FileId(id)}, nil
	}

	dir, name = path.Split(dir)
	dir = path.Clean(dir)
	if dir != "." {
		logrus.Infof("Setting directory: %s", dir)
		_, err = v.DB.Exec("SET_DIR", sqlx.Args{"vault": v.ID, "dir": dir, "group": file.Realm,
			"name": name})
		if err != nil {
			return File{}, core.Error(core.DbError, "cannot set directory %s/%s", dir, name, err)
		}
	}
	file.Id = FileId(id)

	core.End("")
	return file, err
}
