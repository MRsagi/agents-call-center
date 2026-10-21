package cli

import (
	"context"
	"sync"

	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/tray"
)

type serveTrayProvider struct {
	mu       sync.Mutex
	app      *app.App
	httpBind string
	cancel   context.CancelFunc
}

func (p *serveTrayProvider) set(a *app.App, httpBind string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.app = a
	p.httpBind = httpBind
}

func (p *serveTrayProvider) StatusSnapshot() tray.StatusSnapshot {
	p.mu.Lock()
	a := p.app
	p.mu.Unlock()
	if a == nil {
		return tray.StatusSnapshot{}
	}
	return a.StatusSnapshot()
}

func (p *serveTrayProvider) ConsoleHTTPBind() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.httpBind
}

func (p *serveTrayProvider) TrayQuit() {
	if p.cancel != nil {
		p.cancel()
	}
}
