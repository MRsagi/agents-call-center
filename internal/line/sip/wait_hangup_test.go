package sip

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

func testHangupLine(t *testing.T) *Line {
	t.Helper()
	ua, err := sipgo.NewUA(sipgo.WithUserAgent("test-hangup"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ua.Close() })
	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname("127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	dialog := sipgo.NewDialogClientCache(client, sip.ContactHeader{
		Address: sip.Uri{User: "acc", Host: "127.0.0.1"},
	})
	return &Line{
		cfg: Config{
			Logf: func(format string, args ...any) { t.Logf(format, args...) },
		},
		dialog:   dialog,
		hangupCh: make(chan struct{}, 1),
	}
}

func byeRequest() *sip.Request {
	recipient := sip.Uri{User: "acc", Host: "127.0.0.1", Port: 5060}
	req := sip.NewRequest(sip.BYE, recipient)
	viaParams := sip.NewParams()
	viaParams.Add("branch", "z9hG4bKtest")
	req.AppendHeader(&sip.ViaHeader{Host: "127.0.0.1", Port: 5060, Params: viaParams})
	fromParams := sip.NewParams()
	fromParams.Add("tag", "from-tag")
	req.AppendHeader(&sip.FromHeader{Address: sip.Uri{User: "100", Host: "127.0.0.1"}, Params: fromParams})
	toParams := sip.NewParams()
	toParams.Add("tag", "to-tag")
	req.AppendHeader(&sip.ToHeader{Address: sip.Uri{User: "acc", Host: "127.0.0.1"}, Params: toParams})
	req.AppendHeader(sip.NewHeader("Call-ID", "test-call-id"))
	req.AppendHeader(&sip.CSeqHeader{SeqNo: 2, MethodName: sip.BYE})
	return req
}

type stubServerTx struct {
	lastStatus int
}

func (s *stubServerTx) Respond(res *sip.Response) error {
	s.lastStatus = res.StatusCode
	return nil
}
func (s *stubServerTx) Terminate()                       {}
func (s *stubServerTx) OnTerminate(sip.FnTxTerminate) bool { return false }
func (s *stubServerTx) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (s *stubServerTx) Err() error                     { return nil }
func (s *stubServerTx) Acks() <-chan *sip.Request      { return nil }
func (s *stubServerTx) OnCancel(sip.FnTxCancel) bool   { return false }

func TestWaitHangup_ReturnsAfterSignal(t *testing.T) {
	l := testHangupLine(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- l.WaitHangup(ctx) }()

	time.Sleep(20 * time.Millisecond)
	select {
	case l.hangupCh <- struct{}{}:
	default:
		t.Fatal("hangupCh full")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("WaitHangup: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitHangup did not return after signal")
	}
}

func TestWaitHangup_DrainsStaleSignal(t *testing.T) {
	l := testHangupLine(t)
	select {
	case l.hangupCh <- struct{}{}:
	default:
		t.Fatal("hangupCh full")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	err := l.WaitHangup(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v want context.DeadlineExceeded (stale signal must be drained)", err)
	}
}

func TestWaitHangup_IdleTimeout(t *testing.T) {
	prev := idleHangupMaxWait
	idleHangupMaxWait = 25 * time.Millisecond
	t.Cleanup(func() { idleHangupMaxWait = prev })

	l := testHangupLine(t) // dialogSess nil => idle path
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := l.WaitHangup(ctx)
	if err == nil || !strings.Contains(err.Error(), "idle hangup wait timed out") {
		t.Fatalf("err=%v want idle hangup wait timed out", err)
	}
}

func TestHangup_DrainsHangupCh(t *testing.T) {
	l := testHangupLine(t)
	select {
	case l.hangupCh <- struct{}{}:
	default:
		t.Fatal("hangupCh full")
	}
	if err := l.Hangup(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-l.hangupCh:
		t.Fatal("hangupCh still had a signal after Hangup drain")
	default:
	}
}

func TestOnBye_WithWaiterAcceptsUnknownDialog(t *testing.T) {
	l := testHangupLine(t)
	l.hangupWaiters.Add(1)
	defer l.hangupWaiters.Add(-1)

	tx := &stubServerTx{}
	l.onBye(byeRequest(), tx)
	if tx.lastStatus != 200 {
		t.Fatalf("status=%d want 200", tx.lastStatus)
	}
	select {
	case <-l.hangupCh:
	default:
		t.Fatal("expected hangupCh signal when waiters > 0")
	}
}

func TestOnBye_WithoutWaiterReturns481(t *testing.T) {
	l := testHangupLine(t)
	tx := &stubServerTx{}
	l.onBye(byeRequest(), tx)
	if tx.lastStatus != 481 {
		t.Fatalf("status=%d want 481", tx.lastStatus)
	}
	select {
	case <-l.hangupCh:
		t.Fatal("must not signal hangupCh without waiters")
	default:
	}
}
