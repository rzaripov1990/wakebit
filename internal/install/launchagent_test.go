package install

import (
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeInstallDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), "me")
	if d, err := homeInstallDir(home); err != nil || d != filepath.Join(home, "Applications", "Wakebit") {
		t.Fatalf("got %q %v", d, err)
	}
	for _, h := range []string{"", "rel/home"} {
		if d, err := homeInstallDir(h); err == nil {
			t.Errorf("home %q: expected error, got %q", h, d)
		}
	}
}

func TestLaunchAgentPath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "me")
	if p, err := launchAgentPath(home); err != nil || p != filepath.Join(home, "Library", "LaunchAgents", "kz.wakebit.plist") {
		t.Fatalf("got %q %v", p, err)
	}
	if _, err := launchAgentPath("rel"); err == nil {
		t.Fatal("relative home accepted")
	}
}

func TestRenderLaunchAgent(t *testing.T) {
	exe := `/Users/a&b/Applications/Wakebit/<wake"bit'>`
	out := string(renderLaunchAgent(exe))
	if !strings.HasPrefix(out, `<?xml version="1.0" encoding="UTF-8"?>`) ||
		!strings.Contains(out, `<!DOCTYPE plist`) {
		t.Fatalf("missing header:\n%s", out)
	}
	for _, want := range []string{
		"<key>Label</key>", "<string>kz.wakebit</string>",
		"<key>RunAtLoad</key>", "<true/>",
		"<key>ProgramArguments</key>",
		"<string>/Users/a&amp;b/Applications/Wakebit/&lt;wake&#34;bit&#39;&gt;</string>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Well-formed XML whose ProgramArguments string round-trips to exe.
	dec := xml.NewDecoder(strings.NewReader(out))
	dec.Strict = true
	var texts []string
	inString := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("not well-formed: %v", err)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			inString = v.Name.Local == "string"
		case xml.EndElement:
			inString = false
		case xml.CharData:
			if inString {
				texts = append(texts, string(v))
			}
		}
	}
	if len(texts) != 2 || texts[0] != launchAgentLabel || texts[1] != exe {
		t.Fatalf("strings %q", texts)
	}
}
