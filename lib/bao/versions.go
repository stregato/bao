package bao

import (
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

func (s *Bao) ReadVersions(name string) ([]File, error) {
	s.WaitFiles()

	dir, name := path.Split(name)

	rows, err := s.DB.Query("GET_FILE_VERSIONS", sqlx.Args{"store": s.Id, "dir": dir,
		"name": name})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ls []File
	for rows.Next() {
		var id FileId
		var modTimeUnix int64
		var size int64
		var allocatedSize int64
		var flags Flags
		err = rows.Scan(&id, &modTimeUnix, &size, &allocatedSize, &flags)
		if err != nil {
			return nil, err
		}
		isDir := modTimeUnix == 0
		modTime := time.Unix(modTimeUnix, 0)
		if flags&Deleted == 0 {
			ls = append(ls, File{
				Name:          name,
				Size:          size,
				AllocatedSize: allocatedSize,
				ModTime:       modTime,
				IsDir:         isDir,
				Id:            id,
			})
		}
	}
	core.Info("successfully got file versions for %s: %d", name, len(ls))
	return ls, nil
}
