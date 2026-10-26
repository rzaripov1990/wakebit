package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

// acquireSingleInstance takes the named mutex Local\Wakebit; ok is false when
// another instance already holds it.
func acquireSingleInstance() (release func(), ok bool) {
	name, err := windows.UTF16PtrFromString(`Local\Wakebit`)
	if err != nil {
		return func() {}, true
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return func() {}, false
	}
	if err != nil {
		return func() {}, true // fail open
	}
	return func() { windows.CloseHandle(h) }, true
}
