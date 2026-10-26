# Wakebit — design

## Intent
Tray app that keeps a computer from sleeping while the user is away. User launches it, picks a mode (Full / Light), and flips Working/Resting from the tray when leaving the desk. Goal is only "computer doesn't sleep" — not keeping messenger presence green.

## Goals
- Windows first (built and tested here in WSL via cross-compile); macOS code written to build separately and verified later on the user's Mac.
- No admin rights to run or install.
- Instant toggle Working/Resting from tray; tray shows state (green = working, grey = resting; tooltip text).

## Non-goals / anti-goals
- No messenger "online" presence tricks.
- Must not fight the user: if real user input is detected, simulation waits.
- No installer requiring elevation; no services/drivers.

## Behaviour
- **Working** = selected mode active: sleep (system + display) blocked via OS API AND simulation running. **Resting** = nothing blocked, nothing simulated.
- Mode is independent of status: toggling status keeps the chosen mode. Mode and status persist across restarts (config in user profile).
- **Light mode:** every interval (default 60 s, my call) cursor nudges for 1–2 s and returns to its original position.
- **Full mode:** every interval (default 60 s, my call) create/reuse a file in the temp dir, open it in Notepad, find its window by title, focus it, type the current time char by char with short random delays (typewriter). Skip the cycle if user input was seen recently.
- Tray menu: mode (Full/Light), Pause/Resume (status), Autostart, Quit. Left click toggles status.
- Install: exe copies itself to `%LOCALAPPDATA%\Wakebit`; optional autostart via HKCU Run key. macOS: `~/Applications`, autostart via user LaunchAgent.

## Constraints
- Language: Go, single binary (my call; user questioned and accepted). Tray via `fyne.io/systray`. Windows APIs via syscall (no cgo). macOS via cgo, built on a Mac or GitHub Actions.
- macOS Full mode needs the Accessibility permission, granted by the user in System Settings (may prompt for an admin password — OS limitation, accepted by user). Unsigned app: Gatekeeper warning on first launch (right-click → Open).

## Left to the builder
Icon artwork (simple generated coloured dots is fine), config file format, package layout, interval being configurable, test strategy for the OS-specific parts (interfaces + fakes).
