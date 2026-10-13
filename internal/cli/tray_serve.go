package cli

import (
	"context"

	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/tray"
)

type serveTrayProvider struct {
	app    *app.App
	cancel context.CancelFunc
}

func (p *serveTrayProvider) StatusSnapshot() tray.StatusSnapshot {
	return p.app.StatusSnapshot()
}

func (p *serveTrayProvider) TrayQuit() {
	if p.cancel != nil {
		p.cancel()
	}
}
