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

// Quitter is optionally implemented by an Editor that can close the editor
// window it opened (or reused) for the engine's file. The engine calls Quit
// once, before Close, when a Full-mode loop ends (Stop, switch to Resting,
// mode or interval change). Implementations must refuse to touch any window
// that is not the exact editor window for that file, must not leave a "save
// changes?" prompt behind on success, and must return within a few seconds
// (Quit takes no ctx and runs while Stop waits). An error leaves the window
// as it is; the engine only logs it.
type Quitter interface {
	Quit() error
}
