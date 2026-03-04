package vault

import (
	"path"
	"strings"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

type ReadDirOption uint64

func (v *Vault) ReadDir(dir string, since time.Time, afterId FileId, limit int) ([]File, error) {
	core.Start("dir %s, since %v, after %d, limit %d", dir, since, afterId, limit)
	if limit <= 0 {
		limit = 1 << 20
	}

	if since.After(v.lastSyncAt) {
		core.Info("Since parameter %v is after last sync at %v, skipping sync", since, v.lastSyncAt)
		return nil, nil
	}

	now := core.Now()
	if now.Sub(v.lastSyncAt) >= core.DefaultIfZero(v.Config.SyncCooldown, 10*time.Second) {
		// If SyncRelay is not enabled, we need to sync if the last sync time exceeds the cooldown, to make sure we can detect changes in a timely manner. If SyncRelay is enabled, we rely on the relay to notify us of changes, so we don't need to sync here.
		_, err := v.Sync()
		if err != nil {
			return nil, core.Error(core.FileError, "cannot sync vault before reading directory %s", dir, err)
		}
	}

	var ls []File
	seenDirs := map[string]struct{}{}
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
		err = rows.Scan(&file.Id, &file.Name, &file.LocalCopy, &modTimeUnix, &file.Size, &file.AllocatedSize, &file.Flags, &file.Attrs,
			&file.AuthorId, &file.KeyId, &file.StoreDir, &file.StoreName, &file.EcRecipient)
		if err != nil {
			return nil, err
		}
		file.ModTime = time.UnixMilli(modTimeUnix)
		file.IsDir = modTimeUnix == 0

		if file.Flags&Deleted == 0 {
			ls = append(ls, file)
			if file.IsDir {
				seenDirs[file.Name] = struct{}{}
			}
		}
	}

	// Also synthesize directory entries from known file paths so top-level
	// folders (e.g. "replica") appear even when explicit marker rows are absent.
	dirRows, err := v.DB.Query("GET_ALL_DIRS", sqlx.Args{"vault": v.ID})
	if err == nil {
		defer dirRows.Close()
		for dirRows.Next() {
			var knownDir string
			if err := dirRows.Scan(&knownDir); err != nil {
				return nil, err
			}
			child := immediateChildDir(dir, knownDir)
			if child == "" {
				continue
			}
			if _, exists := seenDirs[child]; exists {
				continue
			}
			seenDirs[child] = struct{}{}
			ls = append(ls, File{
				Name:    child,
				IsDir:   true,
				ModTime: time.UnixMilli(0),
			})
		}
	}

	core.End("%d files", len(ls))
	return ls, nil
}

func immediateChildDir(parentDir, knownDir string) string {
	parent := path.Clean(parentDir)
	d := path.Clean(knownDir)
	if d == "." || d == "" {
		return ""
	}
	if parent == "." || parent == "" {
		if strings.HasPrefix(d, "/") {
			d = strings.TrimPrefix(d, "/")
		}
		if d == "" {
			return ""
		}
		if i := strings.IndexByte(d, '/'); i >= 0 {
			return d[:i]
		}
		return d
	}
	prefix := parent + "/"
	if !strings.HasPrefix(d, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(d, prefix)
	if rest == "" {
		return ""
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}
