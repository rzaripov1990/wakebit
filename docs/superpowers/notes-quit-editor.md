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

Both keystrokes are sent only while the HWND is foreground, class `Notepad`,
title matches the file, and no Shift/Ctrl/Alt/Win key is physically held
(Ctrl+Shift+S would open "Save as"). Quit never relocates the window, never
kills a process, and is capped at 3 s because Stop waits for it on the tray
thread while holding the engine mutex. The `Edit`-child detection and Ctrl+W
behaviour are unverified on a real desktop.
