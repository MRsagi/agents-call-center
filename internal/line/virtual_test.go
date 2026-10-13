package line_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/line"
)

func TestVirtualLine_TCPControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	v, err := line.NewVirtual(line.VirtualConfig{
		ControlAddr: addr,
		Logf:        func(format string, args ...any) { t.Logf(format, args...) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	ctx := context.Background()
	if err := v.Ring(ctx); err != nil {
		t.Fatal(err)
	}

	answered := make(chan error, 1)
	go func() {
		answered <- v.WaitAnswer(ctx)
	}()

	if err := virtualSendCommand(v.ControlAddr(), "ANSWER"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-answered:
		if err != nil {
			t.Fatalf("WaitAnswer: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for answer")
	}

	if err := v.Play(ctx, []byte("not-a-wav")); err != nil {
		t.Fatal(err)
	}
	if err := virtualSendCommand(v.ControlAddr(), "DTMF 1"); err != nil {
		t.Fatal(err)
	}
	digits, err := v.CollectDigits(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if digits != "1" {
		t.Fatalf("digits=%q", digits)
	}
	if err := v.Hangup(ctx); err != nil {
		t.Fatal(err)
	}
}

func virtualSendCommand(addr, cmd string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = fmt.Fprintf(conn, "%s\n", cmd)
	return err
}
