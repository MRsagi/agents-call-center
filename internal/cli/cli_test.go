package cli_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/api"
	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/cli"
	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/focus"
	"github.com/MRsagi/agents-call-center/internal/line"
	"github.com/MRsagi/agents-call-center/internal/synth"
)

func TestClient_CallAndStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.HTTPBind = addr
	cfg.RingTimeout = time.Second

	fakeLine := line.NewFake(line.FakeScript{AnswerAfter: time.Millisecond, Digits: "1"})
	a, err := app.NewWithAdapters(cfg, app.Adapters{
		Line:  fakeLine,
		Synth: &synth.Fake{},
		Focus: &focus.Fake{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Stop()

	srv := &http.Server{Addr: addr, Handler: api.NewServer(a).Handler()}
	go func() { _ = srv.ListenAndServe() }()
	defer func() {
		shutdownCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()

	client := cli.NewClient("http://" + addr)

	id, err := client.CreateCall(cli.CreateCallInput{
		Prompt: "approve?",
		App:    "Cursor",
		Title:  "Review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("empty id")
	}

	sum, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !sum.Health.OK || sum.Health.Line != "up" {
		t.Fatalf("health=%+v", sum.Health)
	}
	if sum.Queue.Depth < 0 {
		t.Fatalf("queue depth=%d", sum.Queue.Depth)
	}
}
