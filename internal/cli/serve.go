package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/MRsagi/agents-call-center/internal/api"
	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/domain"
	"github.com/MRsagi/agents-call-center/internal/tray"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the call-center app and HTTP API",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context())
		},
	}
}

func serveDataDir() string {
	if v := strings.TrimSpace(os.Getenv("ACC_DATA_DIR")); v != "" {
		return v
	}
	return config.Default().DataDir
}

func runServe(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	provider := &serveTrayProvider{cancel: runCancel}
	trayStarted := false

	for {
		cfg, _, err := config.LoadEffective(serveDataDir())
		if err != nil {
			return err
		}
		cfg.SynthBackend = config.SynthBackendOS
		cfg.FocusBackend = config.FocusBackendOS

		a, err := app.New(cfg)
		if err != nil {
			return err
		}
		provider.set(a, cfg.HTTPBind)

		if cfg.TrayEnabled && !trayStarted {
			trayStarted = true
			go tray.Run(runCtx, provider)
		}

		if err := a.Start(runCtx); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "acc: line backend %q (set ACC_LINE_BACKEND=sip|virtual|fake)\n", cfg.LineBackend)

		apiSrv := api.NewServer(a)
		if err := apiSrv.ValidateBind(); err != nil {
			a.Stop()
			return err
		}

		restart := make(chan struct{}, 1)
		apiSrv.OnRestart = func() {
			if a.Events != nil {
				a.Events.Publish(domain.Event{Type: domain.EventRestartSoon})
			}
			select {
			case restart <- struct{}{}:
			default:
			}
		}

		srv := &http.Server{
			Addr:    cfg.HTTPBind,
			Handler: apiSrv.Handler(),
		}

		errCh := make(chan error, 1)
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
			close(errCh)
		}()

		fmt.Fprintf(os.Stderr, "acc: listening on http://%s\n", cfg.HTTPBind)

		select {
		case <-runCtx.Done():
			shutdownServe(srv, a)
			return nil
		case <-restart:
			fmt.Fprintf(os.Stderr, "acc: restarting after config save\n")
			shutdownServe(srv, a)
			continue
		case err := <-errCh:
			a.Stop()
			if err != nil {
				return err
			}
			return nil
		}
	}
}

func shutdownServe(srv *http.Server, a *app.App) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	a.Stop()
}
