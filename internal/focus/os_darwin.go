//go:build darwin

package focus

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

type darwinFocuser struct{}

func newOSImpl() Focuser {
	return &darwinFocuser{}
}

func (d *darwinFocuser) Focus(ctx context.Context, target domain.FocusTarget) error {
	app := strings.TrimSpace(target.App)
	if app == "" {
		return fmt.Errorf("focus: app name required")
	}
	title := strings.TrimSpace(target.TitleContains)

	var script strings.Builder
	fmt.Fprintf(&script, `tell application %s to activate`, appleScriptString(app))
	if title != "" {
		proc := appleScriptString(app)
		titleLit := appleScriptString(title)
		fmt.Fprintf(&script, `
tell application "System Events"
	tell process %s
		set frontmost to true
		repeat with w in windows
			if (name of w as text) contains %s then
				perform action "AXRaise" of w
				exit repeat
			end if
		end repeat
	end tell
end tell`, proc, titleLit)
	}
	return runOsascript(ctx, script.String())
}

func (d *darwinFocuser) Type(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(`tell application "System Events"`)
	for _, key := range keys {
		b.WriteByte('\n')
		b.WriteString("\t")
		b.WriteString(darwinKeystrokeLine(key))
	}
	b.WriteString("\nend tell")
	return runOsascript(ctx, b.String())
}

func darwinKeystrokeLine(key string) string {
	switch key {
	case "Enter":
		return "key code 36"
	case "Return":
		return "keystroke return"
	case "Tab":
		return "key code 48"
	case "Escape":
		return "key code 53"
	case "Space":
		return "keystroke space"
	default:
		if len(key) == 1 {
			return fmt.Sprintf("keystroke %s", appleScriptString(key))
		}
		return fmt.Sprintf("keystroke %s", appleScriptString(key))
	}
}

func appleScriptString(s string) string {
	return fmt.Sprintf("%q", s)
}

func runOsascript(ctx context.Context, script string) error {
	if _, err := exec.LookPath("osascript"); err != nil {
		return fmt.Errorf("focus: osascript not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("focus: osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
