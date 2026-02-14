package vault

import (
	"os"
	"path"

	"github.com/stregato/bao/lib/core"
	"github.com/stregato/bao/lib/sqlx"
)

const changeFileName = ".change"

// func (v *Vault) notifyFileChange(baseDir, name string) error {
// 	core.Start("baseDir %s, name %s", baseDir, name)
// 	if v.syncRelayCh != nil {
// 		err := v.notifyChange(name)
// 		if err != nil {
// 			return core.Error(core.GenericError, "failed to notify change in notifyFileChange, name %s: %v", name, err)
// 			// We will continue to update the change file even if the notification fails, because the notification failure does not necessarily mean that the file change is not successful. The change file will be used as a fallback to detect changes when the notification fails.
// 		}
// 		core.End("")
// 		return nil
// 	}
// 	err := v.store.Write(path.Join(baseDir, changeFileName), core.NewBytesReader(nil), nil)
// 	if err != nil {
// 		return core.Error(core.FileError, "failed to write change file in notifyFileChange, name %s: %v", name, err)
// 	}
// 	stat, err := v.store.Stat(path.Join(baseDir, changeFileName))
// 	if err != nil {
// 		return core.Error(core.FileError, "failed to stat change file in notifyFileChange, name %s: %v", name, err)
// 	}
// 	err = v.DB.SetSetting(path.Join(changeFileName, v.ID, v.Realm.String(), baseDir), "", stat.ModTime().Unix(), 0, nil)
// 	if err != nil {
// 		return core.Error(core.DbError, "failed to update modTime in notifyFileChange, name %s: %v", name, err)
// 	}
// 	core.End("")
// 	return nil
// }

func (v *Vault) hasChanged(baseDir string) (bool, error) {
	_, lastChangeTime, _, _, err := v.DB.GetSetting(path.Join(changeFileName, v.ID, v.Realm.String(), baseDir))
	if err == sqlx.ErrNoRows {
		// If there is no setting for the change file, we consider it as changed, because we cannot determine the last change time.
		return true, nil
	}
	if err != nil {
		return false, core.Error(core.DbError, "failed to get last change time in hasChanged, baseDir %s: %v", baseDir, err)
	}
	stat, err := v.store.Stat(path.Join(baseDir, changeFileName))
	if os.IsNotExist(err) {
		// If the change file does not exist, we consider it as changed, because we cannot determine the last change time.
		return true, nil
	}
	if err != nil {
		return false, core.Error(core.FileError, "failed to stat change file in hasChanged, baseDir %s: %v", baseDir, err)
	}
	if stat.ModTime().Unix() > lastChangeTime {
		return true, nil
	} else {
		return false, nil
	}
}
