//go:build darwin

package sysproxy

import (
	"sync"
	"testing"
)

// TestDarwin_ConcurrentEnableDisable ensures that concurrent calls to EnableSOCKS5
// and Disable do not cause data races or panics.
func TestDarwin_ConcurrentEnableDisable(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = EnableSOCKS5("127.0.0.1", 1080)
		}()
		go func() {
			defer wg.Done()
			_ = Disable()
		}()
	}
	wg.Wait()
}
