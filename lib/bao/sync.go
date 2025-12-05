package bao

import (
	"database/sql"
	"path"
	"sync"
	"time"

	"sort"

	"github.com/sirupsen/logrus"
	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
	"github.com/stregato/bao/lib/storage"
)

// Sync synchronizes the filesystem for the specified groups.
// If no groups are specified, it returns an error.
// It returns a list of new files that were added during the synchronization.
// The function ensures that the store is opened before proceeding with synchronization.
func (s *Bao) Sync(groups ...Group) (newFiles []File, err error) {
	core.Start("groups %v", groups)
	if len(groups) == 0 {
		return nil, core.Errorw("no groups provided for synchronization")
	}
	if s.store == nil {
		s.store, err = storage.Open(s.Url)
		if err != nil {
			return nil, core.Errorw("cannot open store with connection URL %s", s.Url, err)
		}
	}

	for _, group := range groups {
		syncFiles, err := s.syncDirs([]Group{group})
		if err != nil {
			return nil, core.Errorw("cannot sync group %s", group, err)
		}
		newFiles = append(newFiles, syncFiles...)
	}

	core.End("%d new files", len(newFiles))
	return newFiles, nil
}

func (s *Bao) syncDirs(groups []Group) ([]File, error) {
	core.Start("groups %v", groups)
	now := time.Now()

	var newFiles []File
	ch := make(chan []File, len(groups))
	for _, group := range groups {
		func(group Group) { // Use a goroutine to handle each group
			if !s.checkAndUpdateExternalChange(group) {
				core.End("no external changes detected, skipping directory synchronization")
				ch <- nil // Send nil to the channel if no changes detected
				return
			}

			baseFolder := path.Join(DataFolder, group.String())
			files, err := s.syncronizeOn(baseFolder)
			ch <- files // Send the files to the channel for collection
			if err != nil {
				core.LogError("cannot synchronize group %s: %v", group, err)
			}
		}(group)
	}
	for range groups {
		files := <-ch
		newFiles = append(newFiles, files...)
	}

	core.End("elapsed %s", time.Since(now))
	return newFiles, nil

}

func (s *Bao) findLastStoreDirIn(baseDir string) (string, error) {
	core.Start("baseDir %s", baseDir)

	var lastStoreDir string
	err := s.DB.QueryRow("GET_LAST_STORE_DIR", sqlx.Args{
		"store":   s.Id,
		"baseDir": baseDir,
	}, &lastStoreDir)
	if err != nil && err != sql.ErrNoRows {
		return "", core.Errorw("cannot get latest sealed file", err)
	}

	core.End("last store dir %s", lastStoreDir)
	return lastStoreDir, nil
}

func (s *Bao) listDirs(path string, min, max string) []string {
	core.Start("path %s, min %s, max %s", path, min, max)
	if path == max {
		core.End("path is already max")
		return []string{path}
	}
	entries, err := s.store.ReadDir(path, storage.Filter{})
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

func (s *Bao) getKnownFilesNames(storeDir string) (map[string]bool, error) {
	core.Start("storeDir %s", storeDir)
	rows, err := s.DB.Query("GET_STORE_NAMES", sqlx.Args{"store": s.Id, "storeDir": storeDir})
	if err == sql.ErrNoRows {
		core.End("no known files in store dir %s", storeDir)
		return nil, nil // No files in sealed directory
	}
	if err != nil {
		return nil, core.Errorw("cannot get files in sealed dir %s", storeDir, err)
	}
	defer rows.Close()
	files := make(map[string]bool)
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, core.Errorw("cannot scan file name in sealed dir %s", storeDir, err)
		}
		files[name] = true
	}

	core.End("retrieved %d names in store dir %s", len(files), storeDir)
	return files, nil
}

func (s *Bao) syncronizeOn(baseDir string) ([]File, error) {
	core.Start("baseDir %s", baseDir)
	var err error
	var newFiles []File

	// 1. Get the last store directory with prefix baseFolder
	lastStoreDir, err := s.findLastStoreDirIn(baseDir)
	if err != nil {
		return nil, core.Errorw("cannot find last store dir", err)
	}

	var minSegment string
	if lastStoreDir != "" {
		minSegment = path.Base(lastStoreDir)
	}
	maxSegment := getSegmentDir(s.Config.SegmentInterval)

	var segments []string
	if minSegment == maxSegment {
		segments = []string{minSegment} // If the latest batch is the same as the current, only use that
	} else {
		segments = s.listDirs(baseDir, minSegment, maxSegment)
	}

	for _, segment := range segments {
		storeDir := path.Join(baseDir, segment)
		ls, err := s.store.ReadDir(path.Join(storeDir, "h"), storage.Filter{})
		if err != nil {
			continue // skip if directory does not exist or is not readable
		}

		knowns, err := s.getKnownFilesNames(storeDir)
		if err != nil {
			return nil, core.Errorw("cannot get known files in sealed dir %s", storeDir, err)
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

				file, err := s.syncronizeFile(storeDir, name)
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
					core.Info("error synchronizing file in batch %s: %v", segment, v)
				}
			}
		}
	}
	core.End("%d new files", len(newFiles))
	return newFiles, nil
}

func (s *Bao) syncronizeFile(storeDir, storeName string) (File, error) {
	core.Start("storeDir %s, storeName %s", storeDir, storeName)

	n := path.Join(storeDir, "h", storeName)
	head, err := storage.ReadFile(s.store, n)
	if err != nil {
		return File{}, core.Errorw("cannot read sealed file %s", n, err)
	}

	file, err := decodeHead(head, s.UserId, s.getKey)
	if err != nil {
		return File{}, core.Errorw("cannot decode file head %s", n, err)
	}

	file.AllocatedSize = int64(len(head)) + file.Size
	file.StoreDir = storeDir
	file.StoreName = storeName
	group, err := s.getGroupFromKey(file.KeyId)
	if err != nil {
		return File{}, core.Errorw("cannot get group from key %d for file %s", file.KeyId, n, err)
	}
	file.Group = group
	file, err = s.writeFileHeadToDB(file)
	if err != nil {
		return File{}, core.Errorw("cannot write file head to DB for %s", n, err)
	}

	s.allocatedSize += file.AllocatedSize
	core.End("file %s, size %d, allocated %d, modTime %s", file.Name, file.Size, file.AllocatedSize, file.ModTime)
	return file, nil
}

func (s *Bao) writeFileHeadToDB(file File) (File, error) {
	core.Start("file %s", file.Name)
	dir, name := path.Split(file.Name)
	dir = path.Clean(dir)
	if file.AllocatedSize == 0 && file.Size > 0 {
		file.AllocatedSize = file.Size
	}

	r, err := s.DB.Exec("SET_FILE", sqlx.Args{
		"store":          s.Id,
		"dir":            dir,
		"storeDir":       file.StoreDir,
		"storeName":      file.StoreName,
		"name":           name,
		"group":          file.Group,
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
		return File{}, core.Errorw("cannot set file %s/%s", dir, name, err)
	}
	id, err := r.LastInsertId()
	if err != nil {
		return File{}, core.Errorw("cannot get last insert id for file %s/%s", dir, name, err)
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
		_, err = s.DB.Exec("SET_DIR", sqlx.Args{"store": s.Id, "dir": dir, "group": file.Group,
			"name": name})
		if err != nil {
			return File{}, core.Errorw("cannot set directory %s/%s", dir, name, err)
		}
	}
	file.Id = FileId(id)

	core.End("")
	return file, err
}

func (s *Bao) ListGroups() ([]Group, error) {
	core.Start("")

	now := time.Now()
	if len(s.groups) > 0 {
		core.End("returning cached groups, count: %d in %s", len(s.groups), time.Since(now))
		return s.groups, nil // Return cached groups if available
	}

	rows, err := s.DB.Query("GET_SCOPES", sqlx.Args{"store": s.Id})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		if len(name) < 32 {
			groups = append(groups, Group(name))
		}
	}
	core.End("%d groups, elapsed %s", len(groups), time.Since(now))
	return groups, nil
}
