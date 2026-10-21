//go:build windows || (cgo && (darwin || linux))

package tray

import (
	"context"
	"log"
	"runtime"
	"time"

	"github.com/getlantern/systray"
)

// consoleBinder supplies the current HTTP bind for Open Console.
type consoleBinder interface {
	ConsoleHTTPBind() string
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
	if len(trayIcon) > 0 {
		systray.SetIcon(trayIcon)
	} else {
		log.Printf("acc: tray icon missing; continuing without SetIcon")
	}
	applySnapshot(provider.StatusSnapshot())

	statusItem := systray.AddMenuItem("", "")
	statusItem.Disable()
	vmItem := systray.AddMenuItem("Voicemails: 0", "")
	vmItem.Disable()

	var openCh <-chan struct{}
	if runtime.GOOS != "linux" {
		openItem := systray.AddMenuItem("Open Console", "Open Console in browser")
		openCh = openItem.ClickedCh
	}

	quitItem := systray.AddMenuItem("Quit", "Stop acc serve")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-openCh:
				bind := "127.0.0.1:8741"
				if b, ok := provider.(consoleBinder); ok {
					if s := b.ConsoleHTTPBind(); s != "" {
						bind = s
					}
				}
				if err := OpenURL(ConsoleURL(bind)); err != nil {
					log.Printf("acc: open console: %v", err)
				}
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
