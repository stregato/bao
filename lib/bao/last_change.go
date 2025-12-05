package bao

// const lastChangeId = ".lastChange"

// func (s *Bao) hasTheFolderChanged(folder string) (hasChanged bool, lastChange int64, err error) {
// 	changeId := path.Join(s.Id, folder, lastChangeId)
// 	_, lastChange, _, _, err = s.DB.GetSetting(changeId)
// 	if err != nil && err != sqlx.ErrNoRows {
// 		return false, 0, core.Errorw(err, "cannot get last change for %s: %v", folder)
// 	}

// 	lastChangeFile := path.Join(folder, ".lastChange")
// 	f, err := s.store.Stat(lastChangeFile)
// 	if err == nil {
// 		if f.ModTime().Unix() > lastChange {
// 			lastChange = f.ModTime().Unix()
// 			core.Info("last change %d is newer than %d, has changed", lastChange, f.ModTime().Unix())
// 		} else {
// 			core.Info("last change %d is older or equal than %d, has not changed", lastChange, f.ModTime().Unix())
// 			return false, lastChange, nil
// 		}

// 	}
// 	if os.IsNotExist(err) {
// 		core.Info("last change file %s does not exist, assuming no changes", lastChangeFile)
// 		return false, lastChange, nil
// 	}

// 	return true, lastChange, nil
// }

// func (s *Bao) alignLastChange(folder string, lastChange int64) error {
// 	changeId := path.Join(s.Id, folder, lastChangeId)
// 	err := s.DB.SetSetting(changeId, "", lastChange, 0, nil)
// 	if err != nil {
// 		return core.Errorw(err, "cannot set last change for %s: %v", folder)
// 	}
// 	core.Info("set last change for %s to %d", folder, lastChange)
// 	return nil
// }

// func (s *Bao) writeLastChange(folder string) {
// 	s.lastChangeLock.Lock()

// 	s.lastChangeScheduledFolders[folder] = true
// 	if s.lastChangeScheduled != nil {
// 		s.lastChangeLock.Unlock()
// 		core.Info("last change for %s already scheduled, skipping", folder)
// 		return
// 	}

// 	s.lastChangeScheduled = make(chan struct{})
// 	s.lastChangeLock.Unlock()
// 	go func() {
// 		core.Info("writing last change for %s", folder)
// 		s.lastChangeLock.Lock()
// 		defer s.lastChangeLock.Unlock()
// 		core.Info("writing last change for %s to store", folder)
// 		for folder := range s.lastChangeScheduledFolders {
// 			changeFile := path.Join(folder, lastChangeId)
// 			err := s.store.Write(changeFile, core.NewBytesReader(nil), nil)
// 			if err != nil {
// 				core.Errorw(err, "cannot write last change for %s: %v", folder)
// 				continue
// 			}
// 			core.Info("wrote last change for %s to %s", folder, changeFile)
// 		}
// 		close(s.lastChangeScheduled)
// 		s.lastChangeScheduled = nil
// 		s.lastChangeScheduledFolders = make(map[string]bool)
// 		core.Info("last change for %s written successfully", folder)
// 	}()
// }

// func (s *Bao) ensureLastScheduledChangesCompleted() {
// 	core.Info("Ensuring all last change scheduled tasks are completed")

// 	s.lastChangeLock.Lock()
// 	ch := s.lastChangeScheduled
// 	s.lastChangeLock.Unlock()
// 	if ch != nil {
// 		core.Info("Waiting for last change scheduled tasks to finish")
// 		<-ch
// 	}
// 	core.Info("All last change scheduled tasks completed")
// }
