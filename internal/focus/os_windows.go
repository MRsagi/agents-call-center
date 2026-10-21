//go:build windows

package focus

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

type windowsFocuser struct{}

func newOSImpl() Focuser {
	return &windowsFocuser{}
}

func (w *windowsFocuser) Focus(ctx context.Context, target domain.FocusTarget) error {
	app := strings.TrimSpace(target.App)
	title := strings.TrimSpace(target.TitleContains)
	if app == "" && title == "" {
		return fmt.Errorf("focus: app name or window title required")
	}
	candidates := []string{}
	if app != "" {
		candidates = append(candidates, app)
	}
	if title != "" && title != app {
		candidates = append(candidates, title)
	}
	var lastErr error
	for _, name := range candidates {
		ps := fmt.Sprintf(
			"$s = New-Object -ComObject WScript.Shell; if (-not $s.AppActivate(%q)) { exit 1 }",
			name,
		)
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("focus: AppActivate %q: %w: %s", name, err, strings.TrimSpace(string(out)))
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("focus: could not activate window")
}

func (w *windowsFocuser) Type(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	var seq strings.Builder
	for _, key := range keys {
		seq.WriteString(windowsSendKey(key))
	}
	ps := fmt.Sprintf(
		"$s = New-Object -ComObject WScript.Shell; $s.SendKeys(%q)",
		seq.String(),
	)
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("focus: SendKeys: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func windowsSendKey(key string) string {
	switch key {
	case "Enter", "Return":
		return "{ENTER}"
	case "Tab":
		return "{TAB}"
	case "Escape":
		return "{ESC}"
	case "Space":
		return " "
	default:
		if chord, ok := windowsChordSendKeys(key); ok {
			return chord
		}
		return sendKeysEscapeLiteral(key)
	}
}

// windowsChordSendKeys maps tokens like "Ctrl+m" to SendKeys "^m".
// Modifier prefixes (^ % +) are emitted unescaped; only the final key is escaped.
func windowsChordSendKeys(key string) (string, bool) {
	parts := strings.Split(key, "+")
	if len(parts) < 2 {
		return "", false
	}
	var mods strings.Builder
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "Ctrl":
			mods.WriteByte('^')
		case "Alt":
			mods.WriteByte('%')
		case "Shift":
			mods.WriteByte('+')
		case "Cmd":
			// Windows has no Cmd; treat like Ctrl for cross-platform chords.
			mods.WriteByte('^')
		default:
			return "", false
		}
	}
	final := parts[len(parts)-1]
	switch final {
	case "Enter", "Return":
		return mods.String() + "{ENTER}", true
	case "Tab":
		return mods.String() + "{TAB}", true
	case "Escape":
		return mods.String() + "{ESC}", true
	case "Space":
		return mods.String() + " ", true
	default:
		return mods.String() + sendKeysEscapeLiteral(final), true
	}
}

func sendKeysEscapeLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		ch := string(r)
		switch ch {
		case "+", "^", "%", "~", "(", ")", "{", "}", "[", "]":
			b.WriteString("{" + ch + "}")
		default:
			b.WriteString(ch)
		}
	}
	return b.String()
}
