//go:build !windows && !(cgo && (darwin || linux))

package tray

import "context"

func runSystray(ctx context.Context, provider StatusProvider) {
	_ = ctx
	_ = provider
}
