package engine

import (
	"context"
	"time"
)

// Platform abstracts the OS-specific operations the engine needs.
type Platform interface {
	PreventSleep(on bool) error
	CursorPos() (x, y int, err error)
	MoveCursor(x, y int) error
	// IdleFor reports the time since the last real user input.
	IdleFor() time.Duration
	// OpenEditor opens the file in Notepad/TextEdit.
	OpenEditor(path string) (Editor, error)
}

// Editor is a handle to an opened editor window.
type Editor interface {
	// Focus finds the window by title and brings it to front; error if gone.
	Focus() error
	// Type types text char by char; it aborts on ctx cancel.
	Type(ctx context.Context, text string, delay func() time.Duration) error
	// Close releases handles only; it does not close the editor.
	Close()
}
