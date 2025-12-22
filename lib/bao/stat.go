package bao

import (
	"os"
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/security"
	"github.com/stregato/bao/lib/sqlx"
)

// Stat retrieves the file information for a given file name from the vault.
func (s *Bao) Stat(name string) (File, error) {
	dir, n := path.Split(name)
	dir = path.Clean(dir)

	var modTimeUnix int64
	var file File

	// Retrieve the file information from the database
	err := s.DB.QueryRow("STAT_FILE", sqlx.Args{"store": s.Id, "name": n, "dir": dir},
		&file.Id, &modTimeUnix, &file.Size, &file.AllocatedSize, &file.Flags, &file.Attrs, &file.Group)
	if err == sqlx.ErrNoRows {
		return File{}, os.ErrNotExist
	}
	if err != nil {
		return File{}, core.Errorw("cannot get file from DB for %s", name, err)
	}
	if file.Flags&Deleted != 0 {
		return File{}, os.ErrNotExist
	}

	file.Name = path.Join(dir, n)
	file.ModTime = time.UnixMilli(modTimeUnix)
	isDir := modTimeUnix == 0

	core.Info("successfully got file info for %s: id=%d, modTime=%s, size=%d, allocated=%d, isDir=%t", name, file.Id, file.ModTime, file.Size, file.AllocatedSize, isDir)
	return file, nil
}

// GetGroup retrieves the group name associated with a given file name.
func (s *Bao) GetGroup(name string) (Group, error) {
	file, found, err := s.queryFileByName(name)
	if err != nil {
		return "", core.Errorw("cannot key id for group %s", name, err)
	}
	if !found {
		return "", os.ErrNotExist
	}

	var group Group
	err = s.DB.QueryRow("GET_GROUP_BY_KEY", sqlx.Args{"idx": file.KeyId}, &group)
	if err != nil {
		return "", core.Errorw("cannot get group for key %d", file.KeyId, err)
	}

	core.Info("successfully got group for %s: %s", name, group)
	return group, nil
}

// GetAuthor retrieves the author ID associated with a given file name.
func (s *Bao) GetAuthor(name string) (security.PublicID, error) {
	file, found, err := s.queryFileByName(name)
	if err != nil {
		return "", core.Errorw("cannot get author idx for %s", name, err)
	}
	if !found {
		return "", os.ErrNotExist
	}

	core.Info("successfully got author for %s: %s", name, file.AuthorId)
	return file.AuthorId, nil
}
