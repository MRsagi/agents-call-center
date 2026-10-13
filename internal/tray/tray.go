package tray

import (
	"context"
	"os"
	"strings"
)

// quitter is implemented by serve wiring to stop the daemon when the user chooses Quit.
type quitter interface {
	TrayQuit()
}

func trayDisabled() bool {
	v := strings.TrimSpace(os.Getenv("ACC_TRAY"))
	return v == "0" || strings.EqualFold(v, "false")
}

// Run starts the system tray when enabled and supported. It blocks until the tray exits
// or ctx is cancelled. When ACC_TRAY=0, Run returns immediately without opening a GUI.
func Run(ctx context.Context, provider StatusProvider) {
	if trayDisabled() || provider == nil {
		return
	}
	runSystray(ctx, provider)
}
