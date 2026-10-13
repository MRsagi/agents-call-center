//go:build windows || (cgo && (darwin || linux))

package tray

import (
	"context"
	"time"

	"github.com/getlantern/systray"
)

// 16×16 ICO (single dark pixel) — minimal placeholder icon for the notification area.
var trayIcon = []byte{
	0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x10, 0x10, 0x00, 0x00, 0x01, 0x00,
	0x20, 0x00, 0x68, 0x04, 0x00, 0x00, 0x16, 0x00, 0x00, 0x00, 0x28, 0x00,
	0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x00, 0x01, 0x00,
	0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00,
}

func runSystray(ctx context.Context, provider StatusProvider) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			systray.Quit()
		case <-done:
		}
	}()

	systray.Run(func() {
		onReady(ctx, provider)
	}, func() {
		close(done)
	})
}

func onReady(ctx context.Context, provider StatusProvider) {
	systray.SetIcon(trayIcon)
	applySnapshot(provider.StatusSnapshot())

	statusItem := systray.AddMenuItem("", "")
	statusItem.Disable()
	vmItem := systray.AddMenuItem("Voicemails: 0", "")
	vmItem.Disable()
	quitItem := systray.AddMenuItem("Quit", "Stop acc serve")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-quitItem.ClickedCh:
				if q, ok := provider.(quitter); ok {
					q.TrayQuit()
				}
				systray.Quit()
				return
			}
		}
	}()

	go pollTray(ctx, provider, statusItem, vmItem)
}

func pollTray(ctx context.Context, provider StatusProvider, statusItem, vmItem *systray.MenuItem) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := provider.StatusSnapshot()
			title, tooltip, statusMenu, voicemailMenu := UpdateTooltip(snap)
			systray.SetTitle(title)
			systray.SetTooltip(tooltip)
			statusItem.SetTitle(statusMenu)
			vmItem.SetTitle(voicemailMenu)
		}
	}
}

func applySnapshot(s StatusSnapshot) {
	title, tooltip, _, _ := UpdateTooltip(s)
	systray.SetTitle(title)
	systray.SetTooltip(tooltip)
}
