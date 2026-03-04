package vault

import (
	"path"
	"time"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

func effectiveRetention(base time.Duration, override time.Duration) time.Duration {
	if base <= 0 {
		base = DefaultRetention
	}
	if override > 0 && override < base {
		return override
	}
	return base
}

func truncateToSecond(t time.Time) time.Time {
	return time.Unix(t.Unix(), 0).UTC()
}

func unixEpochSeconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func timeFromEpochSeconds(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

func (v *Vault) setFileExpiration(storeDir, storeName string, expiresAt time.Time) error {
	if storeDir == "" || storeName == "" || expiresAt.IsZero() {
		return nil
	}
	_, err := v.DB.Exec("SET_FILE_EXPIRATION", sqlx.Args{
		"vault":     v.ID,
		"storeDir":  storeDir,
		"storeName": storeName,
		"expiresAt": unixEpochSeconds(expiresAt),
	})
	if err != nil {
		return core.Error(core.DbError, "cannot set expiration for %s/%s", storeDir, storeName, err)
	}
	return nil
}

func (v *Vault) cleanupExpiredFiles(now time.Time) (int64, error) {
	rows, err := v.DB.Query("GET_EXPIRED_FILE_EXPIRATIONS", sqlx.Args{
		"vault":     v.ID,
		"expiresAt": unixEpochSeconds(now),
		"limit":     5000,
	})
	if err != nil {
		return 0, core.Error(core.DbError, "cannot query expired files", err)
	}
	defer rows.Close()

	var deleted int64
	for rows.Next() {
		var storeDir, storeName string
		if err := rows.Scan(&storeDir, &storeName); err != nil {
			return deleted, core.Error(core.DbError, "cannot scan expired file entry", err)
		}

		// Best-effort store cleanup for both head and body.
		_ = v.store.Delete(path.Join(storeDir, "h", storeName))
		_ = v.store.Delete(path.Join(storeDir, "b", storeName))

		if _, err := v.DB.Exec("DELETE_FILES_BY_STORE_OBJECT", sqlx.Args{
			"vault":     v.ID,
			"storeDir":  storeDir,
			"storeName": storeName,
		}); err != nil {
			return deleted, core.Error(core.DbError, "cannot delete file rows for %s/%s", storeDir, storeName, err)
		}

		if _, err := v.DB.Exec("DELETE_FILE_EXPIRATION", sqlx.Args{
			"vault":     v.ID,
			"storeDir":  storeDir,
			"storeName": storeName,
		}); err != nil {
			return deleted, core.Error(core.DbError, "cannot delete expiration row for %s/%s", storeDir, storeName, err)
		}
		deleted++
	}
	return deleted, nil
}
