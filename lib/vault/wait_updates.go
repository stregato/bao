package vault

import "time"

func (v *Vault) WaitUpdates(timeout time.Duration) bool {
	done := make(chan bool, 1)

	go func() {
		v.newFiles.L.Lock()
		v.newFiles.Wait()

		// Check if this wake was due to interrupt (still holding lock)
		interrupted := v.interrupted
		v.interrupted = false
		v.newFiles.L.Unlock()

		done <- !interrupted
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(timeout):
		return false // timeout
	}
}

func (v *Vault) InterruptWait() {
	v.newFiles.L.Lock()
	v.interrupted = true
	v.newFiles.L.Unlock()
	v.newFiles.Broadcast()
}
