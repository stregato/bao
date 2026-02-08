package vault

import (
	"path"
	"strconv"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

func (v *Vault) Versions(name string) ([]File, error) {
	v.waitFiles()

	dir, name := path.Split(name)
	dir = path.Clean(dir)

	rows, err := v.DB.Query("GET_FILE_VERSIONS", sqlx.Args{"vault": v.ID, "dir": dir,
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
		modTime := time.UnixMilli(modTimeUnix)
		if flags&Deleted == 0 {
			ls = append(ls, File{
				Name:          "", // Will be set below after we know total count
				Size:          size,
				AllocatedSize: allocatedSize,
				ModTime:       modTime,
				IsDir:         isDir,
				Id:            id,
			})
		}
	}

	// Set names with path:offset format to match GET_FILE_BY_NAME ordering
	// Both Versions() and GET_FILE_BY_NAME use ASC ordering (oldest→newest)
	// So offset directly maps to index: oldest at index 0 gets offset 0
	fullPath := path.Join(dir, name)
	for i := range ls {
		ls[i].Name = fullPath + ":" + strconv.FormatUint(uint64(i), 10)
	}

	core.Info("successfully got file versions for %s: %d", name, len(ls))
	return ls, nil
}
