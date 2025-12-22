package sqlx

import (
	"github.com/stregato/bao/lib/core"
)

// SetSetting sets a setting in the vault_settings table.
func (db *DB) SetSetting(id, s string, i int64, f float64, b []byte) error {
	_, err := db.Exec("SET_SETTING", Args{"id": id,
		"valueAsString": s, "valueAsInt": i,
		"valueAsReal": f, "valueAsBlob": b})
	if err != nil {
		return core.Errorw("cannot set setting %s", id, err)
	}
	db.settingCacheMu.Lock()
	db.settingCache[id] = struct {
		s string
		i int64
		f float64
		b []byte
	}{s, i, f, b}
	db.settingCacheMu.Unlock()

	core.Info("successfully set setting %s = %s, %d, %f, %x", id, s, i, f, b)
	return err
}

// GetSetting retrieves a setting from the vault_settings table.
func (db *DB) GetSetting(id string) (s string, i int64, f float64, b []byte, err error) {
	db.settingCacheMu.RLock()
	defer db.settingCacheMu.RUnlock()
	if cached, found := db.settingCache[id]; found {
		core.Info("using cached setting %s = %s, %d, %f, %x", id, cached.s, cached.i, cached.f, cached.b)
		return cached.s, cached.i, cached.f, cached.b, nil
	}

	err = db.QueryRow("GET_SETTING", Args{"id": id},
		&s, &i, &f, &b)
	if err == ErrNoRows {
		return "", 0, 0, nil, ErrNoRows
	}
	if err != nil {
		return "", 0, 0, nil, core.Errorw("cannot get setting %s", id, err)
	}

	db.settingCache[id] = struct {
		s string
		i int64
		f float64
		b []byte
	}{s, i, f, b}

	core.Info("successfully got setting %s = %s, %d, %f, %x", id, s, i, f, b)
	return s, i, f, b, err
}
