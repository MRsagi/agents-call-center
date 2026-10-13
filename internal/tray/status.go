package tray

import "fmt"

// StatusSnapshot is a point-in-time view of daemon health for the tray.
type StatusSnapshot struct {
	QueueDepth      int
	VoicemailCount  int
	LineUp          bool
	LastCallSummary string
}

// StatusProvider supplies fresh snapshots for tray polling.
type StatusProvider interface {
	StatusSnapshot() StatusSnapshot
}

// lineWord returns "up" or "down" for tooltip formatting.
func lineWord(up bool) string {
	if up {
		return "up"
	}
	return "down"
}

// FormatTooltip returns the tray title/tooltip: acc · queue N · vm M · line up|down.
func FormatTooltip(s StatusSnapshot) string {
	return fmt.Sprintf("acc · queue %d · vm %d · line %s", s.QueueDepth, s.VoicemailCount, lineWord(s.LineUp))
}

// UpdateTooltip derives all user-visible tray strings from a snapshot.
func UpdateTooltip(s StatusSnapshot) (title, tooltip, statusMenu, voicemailMenu string) {
	title = FormatTooltip(s)
	tooltip = title
	statusMenu = s.LastCallSummary
	if statusMenu == "" {
		statusMenu = "No recent calls"
	}
	voicemailMenu = fmt.Sprintf("Voicemails: %d", s.VoicemailCount)
	return title, tooltip, statusMenu, voicemailMenu
}
