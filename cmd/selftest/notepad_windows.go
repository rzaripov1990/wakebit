//go:build windows

package main

import winplat "wakebit/internal/platform/windows"

// editorWindowOpen reports whether a Notepad window for path is open; ok is
// false where the check is not available.
func editorWindowOpen(path string) (open, ok bool) { return winplat.EditorWindowOpen(path), true }
