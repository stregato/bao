package vault

import "time"

func (v *Vault) WaitUpdates(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	v.newFiles.L.Lock()
	defer v.newFiles.L.Unlock()

	startSeq := v.updateSeq

	for {
		if v.interrupted {
			v.interrupted = false
			return false
		}
		if v.updateSeq != startSeq {
			return true
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}

		// Wake the cond on timeout so Wait() can re-check conditions.
		timer := time.AfterFunc(remaining, func() {
			v.newFiles.L.Lock()
			v.newFiles.Broadcast()
			v.newFiles.L.Unlock()
		})
		v.newFiles.Wait()
		timer.Stop()
	}
}

func (v *Vault) signalUpdateLocked() {
	v.updateSeq++
	v.newFiles.Broadcast()
}

func (v *Vault) signalUpdate() {
	v.newFiles.L.Lock()
	v.signalUpdateLocked()
	v.newFiles.L.Unlock()
}

func (v *Vault) InterruptWait() {
	v.newFiles.L.Lock()
	v.interrupted = true
	v.newFiles.Broadcast()
	v.newFiles.L.Unlock()
}
