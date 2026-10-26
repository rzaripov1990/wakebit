# Wakebit

[English](README.md) | [Русский](README_RU.md)

A Go tray app that keeps your computer (system and display) from going to sleep while you are away from your desk. It has one goal: preventing sleep. Keeping a "green" presence status in messengers is not a goal.

## Modes and statuses

Mode and status are independent and persist between launches.

- **Working** — the selected mode is active: sleep is blocked through the OS and activity is simulated.
- **Resting** — nothing is blocked and nothing is simulated.

Modes (default is Light, interval 60 s):

- **Light** — once per interval the cursor moves for 1–2 s and returns to where it was. If you move the mouse yourself during the nudge, the nudge is aborted and the cursor is not restored.
- **Full** — once per interval a file `wakebit-time.txt` is created/reused in the temp folder, opened in Notepad, the window is focused and the current time is typed character by character with random pauses. When Full mode ends (pause/"Resting", mode change, quit, or an interval change in the config file — the interval cannot be changed from the tray), on Windows Wakebit briefly brings exactly that Notepad window to the foreground, saves the file (Ctrl+S) and closes it without a save prompt: classic Notepad by closing the window (WM_CLOSE), tabbed Windows 11 Notepad by closing only our tab (Ctrl+W). So on pause and quit Notepad may flash on top of other windows for a moment. Wakebit never acts while you are typing: before every keystroke it waits for 300 ms without keyboard or mouse input and re-checks that the window is Notepad showing our file, that it is in the foreground and that no modifier key is held. The window is closed only after the `*` has disappeared from the title (file saved). Other windows and tabs are not touched; if bringing the window to the foreground safely fails (within ~0.5 s), the window no longer shows our file, the file was not saved (read-only, a "Save As" dialog appeared) or no pause in your input occurred within ~3 s, Wakebit gives up, leaves the window (and dialog) as is and logs the reason. On macOS TextEdit is not closed yet.

In both modes a cycle is skipped while the user is active: actions start only after 5 s without input.

## Usage

Tray icon: green — working, grey — resting; the tooltip reads "Wakebit: работает (Полный)" or "Wakebit: отдыхает" (the UI is in Russian).

- Left click toggles working/resting.
- Right click opens the menu: "Режим: Полный" (Full), "Режим: Лёгкий" (Light), "Пауза" / "Продолжить" (Pause / Resume — same as left click), "Автозапуск" (Autostart, checkbox), "Выход" (Quit).

## Install and uninstall (Windows)

No administrator rights needed.

```
wakebit.exe --install
wakebit.exe --uninstall
```

`--install` copies the exe to `%LOCALAPPDATA%\Wakebit`, enables autostart (the value `Wakebit` under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`) and starts the installed copy. If the installed copy is already running, quit it from the tray and repeat.

`--uninstall` disables autostart and removes the install folder. `--install` and `--uninstall` cannot be combined. Autostart can also be toggled with the "Автозапуск" menu item.

Single-instance guard: Windows uses the named mutex `Local\Wakebit`; macOS (and Linux) use an exclusive `flock` on a `wakebit.lock` file next to `config.json` (e.g. `~/Library/Application Support/Wakebit/wakebit.lock`). A second instance exits immediately and silently. The lock file is not removed on exit — that is fine: the lock is held by an open descriptor and the OS releases it when the process ends (including a crash). If the folder or lock file cannot be created, the app starts without the guard and logs the reason.

## Configuration

The file is created when settings change:

- Windows: `%AppData%\Wakebit\config.json`
- macOS: `~/Library/Application Support/Wakebit/config.json`

```json
{
  "Mode": "light",
  "Working": false,
  "IntervalSec": 60
}
```

`Mode` is `light` or `full`; `Working` is the current status; `IntervalSec` is the interval in seconds (at most 86400; larger values are clamped to 86400). A missing file, broken JSON or invalid values are replaced with defaults.

## Build

```
GOOS=windows go build -ldflags "-H=windowsgui" -o dist/wakebit.exe .
```

Go 1.22+ is required. GitHub Actions (`.github/workflows/build.yml`) runs vet and tests on every push/PR and publishes the artifacts `wakebit-windows` (`dist/wakebit.exe`) and `wakebit-macos` (`dist/wakebit`); the `linux` job checks tests and the Windows cross-build.

## Manual check on Windows

`selftest` runs one Full-mode cycle on the real platform (1 s interval):

```
GOOS=windows go build -o dist/selftest.exe ./cmd/selftest
selftest.exe -d 12s
```

`-d` is how long to run (default 8 s). Do not touch the mouse or keyboard: a cycle starts only after ≥5 s of idle. Expected: Notepad opens `wakebit-time.txt` (the file contains the open time and a newline); Notepad puts the caret at the start of the file, so the typed time lands at the very start of the first line, run together with the written time (e.g. `16:05:1216:05:07`), and the times of later cycles are typed one after another without a newline. At the end `wakebit-time.txt` is saved, the Notepad window (or tab) is closed without a save prompt, sleep blocking is released, and selftest reports whether a window with the file is still open.

## Verification status

- **Windows:** verified on a real Windows 11 desktop with `selftest`: Notepad opens, the time is typed, and on stop the file is saved and the window closes without a prompt. The tray, installation, registry autostart, closing when the user has other unsaved tabs, and Pause/Quit from the tray have **not** been exercised on a real desktop yet. Pure logic is covered by unit tests, which pass.
- **macOS:** the code has **never been compiled or run** — **NOT VERIFIED** until the first macOS CI run and a manual check on a Mac.

## Limitations and known issues

- macOS: Full mode needs the Accessibility permission, which the user grants in System Settings; the system may ask for an administrator password. Light mode works without it but may not reset the "idle" status inside apps like Teams/Slack.
- macOS: the first `osascript` calls trigger Automation permission prompts.
- macOS: the app is unsigned, so Gatekeeper shows a warning — open it via right click → Open.
- The macOS CI build (`macos-latest`) is arm64 only.
- Windows 11 tabbed Notepad: our tab must be the active one.
- Windows: the process declares itself DPI-aware at the very start (per-monitor v2 via `SetProcessDpiAwarenessContext`, `SetProcessDPIAware` on older Windows, no manifest) so cursor coordinates and screen metrics are in physical pixels and the cursor returns exactly. This has **not yet been verified** on a real mixed-DPI desktop.
- Windows `--uninstall` leaves the install folder if the running exe is the installed copy (a running file cannot be deleted).
- Windows: if Full mode did not get a Notepad window (~10 s), the next 3 cycles are skipped so `notepad.exe` is not started every interval.
- Windows: the "Автозапуск" checkbox enabled from a non-installed copy points at that copy (not at `%LOCALAPPDATA%\Wakebit`); the checkbox only checks that the `Wakebit` registry value exists, not where it points.
- Windows: `--install` and `--uninstall` do not report success (a message is shown only on error).
- Windows: `--uninstall` does not stop a running instance (quit via the tray) and does not delete the `%AppData%\Wakebit` config.
- Windows: when Full mode stops, Wakebit saves the temp file and closes its window/tab (up to ~3 s; the tray menu does not respond meanwhile). If the app is killed or crashes, the Notepad window stays open with an unsaved buffer — Notepad may ask to save on shutdown. If closing fails (the window shows another file, focus was taken, a dialog is open), the window stays too and the reason is logged; the Notepad process is never killed.
- Windows: Notepad's title separates the file name and app name with a hyphen, en dash or em dash depending on version/locale (Windows 11 shows `wakebit-time.txt — Блокнот`); all three are accepted.
- Windows: classic and tabbed Notepad are told apart by the presence of a child `Edit` window; this and closing the tab with Ctrl+W have not been verified on a real desktop.
- macOS: the TextEdit window is not closed after Full mode.
- Still to check manually on a real desktop:
  - Windows 10 with hidden file extensions (title `wakebit-time - Блокнот`); the ALT-tap focus fallback; the Light-mode nudge with real mouse movement.
  - Windows 11 tabbed Notepad with the user's unsaved tabs, our tab active / inactive — only our tab closes, the user's tabs and their contents are untouched, no save prompts.
  - Pause by clicking the tray icon and via the menu, and Quit — the window/tab closes and sleep blocking is released.
  - Read-only `wakebit-time.txt` — Ctrl+S does not save, the window (and any dialog) stays open, the reason is in the log.
  - Minimized Notepad — it is restored and closed (or Wakebit gives up with an error without pressing anything).
  - Start typing in Notepad during a cycle, then immediately press Pause — none of Wakebit's keystrokes mix with yours.
  - Cursor return across several monitors with different scaling (mixed DPI).
  - Tray icon and menu at 150% scaling and above.
- The goal is only to prevent sleep, not messenger presence.
