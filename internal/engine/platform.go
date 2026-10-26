package engine

import (
	"context"
	"errors"
	"time"
)

// ErrEditorGone is returned (possibly wrapped) by Editor.Focus or Editor.Type
// when the editor window for the file definitely no longer exists. The engine
// then drops the handle (Close, without Quit) and opens the editor again on a
// later cycle. Any other Focus/Type error is treated as transient for editors
// that implement Quitter: the handle is kept (so Quit can still close the
// window when Full mode ends) and Focus is retried next cycle. Editors without
// Quitter are dropped on any error.
var ErrEditorGone = errors.New("editor window is gone")

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
	// Focus finds the window by title and brings it to front. It returns an
	// error wrapping ErrEditorGone if the window no longer exists.
	Focus() error
	// Type types text char by char; it aborts on ctx cancel. It returns an
	// error wrapping ErrEditorGone if the window no longer exists.
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
