package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MRsagi/agents-call-center/internal/api"
	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/config"
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

func runServe(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	cfg := config.Default()
	config.ApplyEnv(&cfg)
	cfg.SynthBackend = config.SynthBackendOS
	cfg.FocusBackend = config.FocusBackendOS
	a, err := app.New(cfg)
	if err != nil {
		return err
	}

	go tray.Run(runCtx, &serveTrayProvider{app: a, cancel: runCancel})

	if err := a.Start(runCtx); err != nil {
		return err
	}
	defer a.Stop()

	fmt.Fprintf(os.Stderr, "acc: line backend %q (set ACC_LINE_BACKEND=sip|virtual|fake)\n", cfg.LineBackend)

	apiSrv := api.NewServer(a)
	if err := apiSrv.ValidateBind(); err != nil {
		return err
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
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	a.Stop()
	return nil
}
