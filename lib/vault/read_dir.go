package vault

import (
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

type ReadDirOption uint64

func (v *Vault) ReadDir(dir string, since time.Time, afterId int64, limit int) ([]File, error) {
	core.Start("dir %s, since %v, after %d, limit %d", dir, since, afterId, limit)
	if limit <= 0 {
		limit = 1 << 20
	}
	now := core.Now()
	if now.Sub(v.lastSyncAt) >= v.Config.SyncCooldown {
		_, err := v.Sync()
		if err != nil {
			return nil, core.Error(core.FileError, "cannot sync vault before reading directory %s", dir, err)
		}
	}

	var ls []File

	dir = path.Clean(dir)
	modTimeSince := since.Unix()
	rows, err := v.DB.Query("GET_FILES_IN_DIR", sqlx.Args{"vault": v.ID, "dir": dir,
		"since": modTimeSince, "afterId": afterId, "limit": limit})
	if err != nil {
		return nil, core.Error(core.DbError, "cannot get files from DB for directory %s", dir, err)
	}
	defer rows.Close()
	for rows.Next() {
		var file File
		var modTimeUnix int64
		err = rows.Scan(&file.Id, &file.Name, &file.Realm, &file.LocalCopy, &modTimeUnix, &file.Size, &file.AllocatedSize, &file.Flags, &file.Attrs,
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
