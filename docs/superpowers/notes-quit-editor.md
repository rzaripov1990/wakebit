# Closing the Full-mode Notepad window (Editor.Quit)

Deviation from the original design (WM_CLOSE to the HWND): Windows 11 Notepad
opens files as tabs in an existing window by default, so the HWND the engine
holds can also contain the user's own documents. WM_CLOSE would close all of
them (or raise save prompts for them). Quit therefore:

- classic Notepad (has a direct child window of class `Edit`): Ctrl+S, then
  PostMessage(WM_CLOSE); done when the HWND is gone.
- tabbed Notepad (no `Edit` child): Ctrl+S, then Ctrl+W, which closes only the
  active tab (verified to be ours right before); done when the HWND is gone or
  its title no longer names our file.

Safety rules (see `editor.Quit` and the pure helpers in `helpers.go`):

- Quit acts only on the HWND the editor already holds; it never searches for
  another window. It does not call `Focus`: a Quit-only helper restores a
  minimized window with `ShowWindowAsync`, calls `SetForegroundWindow` (no
  `AttachThreadInput`, no ALT tap, no synchronous `ShowWindow` /
  `BringWindowToTop`) and gives up if the window is not foreground within
  500 ms.
- Before Ctrl+S and again before the close step it waits for a 300 ms quiet
  period (`IdleFor`, which ignores our own injections) and then re-checks:
  class `Notepad`, title names our file, HWND is foreground, no
  Shift/Ctrl/Alt/Win down (`GetAsyncKeyState` also sees injected keys; the
  quiet period is what covers the user typing).
- After Ctrl+S it polls until the title loses its leading `*`; if it never
  does (read-only/locked file, Save As or error dialog) nothing is closed.
  Windows 11 Notepad may not show `*` in the window title at all; then the
  close step follows immediately and relies on Notepad processing Ctrl+S
  before Ctrl+W (same input queue). Unverified.
- Success means the HWND is gone, or its title no longer names our file and
  a fresh enumeration finds no Notepad window showing our file. A tab switch
  alone is not reported as success.
- One 3 s deadline bounds every wait (Stop waits for Quit on the tray thread
  while holding the engine mutex). Quit never kills a process.

Engine side: Focus/Type errors no longer drop a Quitter editor (so Quit
still runs on Stop); only errors wrapping `engine.ErrEditorGone` drop it.

The `Edit`-child detection, the `*` title behaviour and Ctrl+W are
unverified on a real desktop.
