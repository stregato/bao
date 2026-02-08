package vault

import (
	"github.com/stregato/bao/lib/core"
)

func (v *Vault) Close() error {
	core.Start("closing store %s", v.ID)
	if v.housekeepingTicker != nil {
		v.housekeepingTicker.Stop()
	}
	if v.stopSyncRelay != nil {
		close(v.stopSyncRelay)
	}

	openedStashesMu.Lock()
	for i, opened := range openedStashes {
		if opened == v {
			openedStashes = append(openedStashes[:i], openedStashes[i+1:]...)
			break
		}
	}
	openedStashesMu.Unlock()

	core.Info("successfully closed vault %s", v.ID)
	core.End("")
	return nil
}
