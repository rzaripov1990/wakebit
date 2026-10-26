package winplat

import "sync"

// ensureOnce returns a function that runs fn on its first call only; later
// and concurrent calls wait for that first run and then do nothing.
func ensureOnce(fn func()) func() {
	var once sync.Once
	return func() { once.Do(fn) }
}
