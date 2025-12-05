package bao

import (
	"github.com/stregato/bao/lib/core"
)

func (s *Bao) Close() error {
	core.Start("closing store %s", s.Id)
	if s.housekeepingTicker != nil {
		s.housekeepingTicker.Stop()
	}
	// if s.blockChainTicker != nil {
	// 	s.blockChainTicker.Stop()
	// }

	// ch := s.lastChangeScheduled
	// if ch != nil {
	// 	<-ch
	// }

	err := s.store.Close()
	if err != nil {
		return core.Errorw("cannot close store", err)
	}

	openedStashesMu.Lock()
	for i, opened := range openedStashes {
		if opened == s {
			openedStashes = append(openedStashes[:i], openedStashes[i+1:]...)
			break
		}
	}
	openedStashesMu.Unlock()

	core.Info("successfully closed stash %s", s.Id)
	core.End("")
	return nil
}
