package install

// Pure helpers for the macOS install (see install_darwin.go); untagged so
// they are unit-tested on every OS.

import (
	"bytes"
	"encoding/xml"
	"errors"
	"path/filepath"
)

// launchAgentLabel is the launchd label and the plist file's base name.
const launchAgentLabel = "kz.wakebit"

// homeInstallDir returns ~/Applications/Wakebit for an absolute home.
func homeInstallDir(home string) (string, error) {
	if !filepath.IsAbs(home) {
		return "", errors.New("install: cannot determine an absolute home directory")
	}
	return filepath.Join(home, "Applications", "Wakebit"), nil
}

// launchAgentPath returns ~/Library/LaunchAgents/kz.wakebit.plist for an
// absolute home.
func launchAgentPath(home string) (string, error) {
	if !filepath.IsAbs(home) {
		return "", errors.New("install: cannot determine an absolute home directory")
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

// renderLaunchAgent returns a user LaunchAgent plist that starts exe at
// login (RunAtLoad). All text is XML-escaped.
func renderLaunchAgent(exe string) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>`)
	_ = xml.EscapeText(&b, []byte(launchAgentLabel))
	b.WriteString(`</string>
	<key>ProgramArguments</key>
	<array>
		<string>`)
	_ = xml.EscapeText(&b, []byte(exe))
	b.WriteString(`</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`)
	return b.Bytes()
}
