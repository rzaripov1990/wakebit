//go:build !windows

package main

// acquireSingleInstance is a no-op outside Windows.
func acquireSingleInstance() (release func(), ok bool) { return func() {}, true }
