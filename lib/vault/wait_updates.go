package vault

import "time"

func (v *Vault) WaitUpdates(timeout time.Duration) bool {
	done := make(chan bool, 1)

	go func() {
		v.newFiles.L.Lock()
		v.newFiles.Wait()
		v.newFiles.L.Unlock()
		done <- true
	}()

	select {
	case <-done:
		return true // file arrived
	case <-time.After(timeout):
		return false // timeout
	}
}
