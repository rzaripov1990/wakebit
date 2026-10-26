//go:build !windows

package main

// editorWindowOpen is not implemented outside Windows.
func editorWindowOpen(string) (open, ok bool) { return false, false }
