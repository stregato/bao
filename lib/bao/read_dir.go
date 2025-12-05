package bao

import (
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

type ReadDirOption uint64

func (s *Bao) ReadDir(dir string, since time.Time, afterId int64, limit int) ([]File, error) {
	core.Start("dir %s, since %v, after %d, limit %d", dir, since, afterId, limit)
	if limit <= 0 {
		limit = 1 << 20
	}

	var ls []File

	dir = path.Clean(dir)
	modTimeSince := since.Unix()
	rows, err := s.DB.Query("GET_FILES_IN_DIR", sqlx.Args{"store": s.Id, "dir": dir,
		"since": modTimeSince, "afterId": afterId, "limit": limit})
	if err != nil {
		return nil, core.Errorw("cannot get files from DB for directory %s", dir, err)
	}
	defer rows.Close()
	for rows.Next() {
		var file File
		var modTimeUnix int64
		err = rows.Scan(&file.Id, &file.Name, &file.Group, &file.LocalCopy, &modTimeUnix, &file.Size, &file.AllocatedSize, &file.Flags, &file.Attrs,
			&file.AuthorId, &file.KeyId, &file.StoreDir, &file.StoreName)
		if err != nil {
			return nil, err
		}
		file.ModTime = time.UnixMilli(modTimeUnix)
		file.IsDir = modTimeUnix == 0

		if file.Flags&Deleted == 0 {
			ls = append(ls, file)
		}
	}

	core.End("%d files", len(ls))
	return ls, nil
}
