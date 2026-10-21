package focus

import (
	"errors"
	"fmt"
	"strings"
)

func KeysForDigits(digits string) []string {
	if digits == "" {
		return nil
	}
	keys := make([]string, 0, len(digits)+1)
	for _, r := range digits {
		keys = append(keys, string(r))
	}
	keys = append(keys, "Enter")
	return keys
}

func KeysForChord(chord string) ([]string, error) {
	c := strings.ToLower(strings.TrimSpace(chord))
	if c == "" {
		return nil, errors.New("empty chord")
	}
	parts := strings.Split(c, "+")
	if len(parts) == 0 {
		return nil, errors.New("invalid chord")
	}
	key := parts[len(parts)-1]
	mods := parts[:len(parts)-1]
	var b strings.Builder
	for _, m := range mods {
		switch m {
		case "ctrl", "control":
			b.WriteString("Ctrl+")
		case "alt":
			b.WriteString("Alt+")
		case "shift":
			b.WriteString("Shift+")
		case "cmd", "meta", "super":
			b.WriteString("Cmd+")
		default:
			return nil, fmt.Errorf("unknown modifier %q", m)
		}
	}
	b.WriteString(key)
	return []string{b.String()}, nil
}
