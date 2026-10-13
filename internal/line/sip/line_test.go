package sip_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	sipline "github.com/MRsagi/agents-call-center/internal/line/sip"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/icholy/digest"
)

func TestLine_RegisterReady(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("sip register integration can be flaky on shared CI")
	}

	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	bind := ln.LocalAddr().String()

	line, err := sipline.New(sipline.Config{
		Bind:       bind,
		Password:   "acc",
		Realm:      "agents-call-center",
		Extension:  "100",
		PublicHost: "127.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = line.Close() })

	if line.Ready() {
		t.Fatal("expected not ready before REGISTER")
	}

	serverAddr := line.ListenAddr()
	if serverAddr == "" {
		t.Fatal("listen addr empty")
	}

	if err := registerClient(t, serverAddr, "100", "100", "acc"); err != nil {
		t.Fatalf("register: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for !line.Ready() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !line.Ready() {
		t.Fatal("expected Ready() true after REGISTER")
	}
}

func registerClient(t *testing.T, serverAddr, ext, user, pass string) error {
	t.Helper()
	host, port, err := net.SplitHostPort(serverAddr)
	if err != nil {
		return err
	}
	ua, err := sipgo.NewUA(sipgo.WithUserAgent("test-ata"))
	if err != nil {
		return err
	}
	defer ua.Close()
	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname("127.0.0.1"))
	if err != nil {
		return err
	}
	defer client.Close()

	recipient := sip.Uri{User: ext, Host: host, Port: mustAtoi(port)}
	req := sip.NewRequest(sip.REGISTER, recipient)
	req.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<sip:%s@127.0.0.1:5099>", ext)))
	req.AppendHeader(sip.NewHeader("Expires", "3600"))
	req.SetTransport("UDP")
	req.SetDestination(serverAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := client.TransactionRequest(ctx, req, sipgo.ClientRequestRegisterBuild)
	if err != nil {
		return err
	}
	defer tx.Terminate()

	res, err := waitResponse(tx)
	if err != nil {
		return err
	}
	if res.StatusCode == 401 {
		www := res.GetHeader("WWW-Authenticate")
		if www == nil {
			return fmt.Errorf("missing WWW-Authenticate")
		}
		chal, err := digest.ParseChallenge(www.Value())
		if err != nil {
			return err
		}
		cred, err := digest.Digest(chal, digest.Options{
			Method:   "REGISTER",
			URI:      recipient.Addr(),
			Username: user,
			Password: pass,
		})
		if err != nil {
			return err
		}
		req2 := req.Clone()
		req2.RemoveHeader("Via")
		req2.AppendHeader(sip.NewHeader("Authorization", cred.String()))
		tx2, err := client.TransactionRequest(ctx, req2, sipgo.ClientRequestIncreaseCSEQ, sipgo.ClientRequestAddVia)
		if err != nil {
			return err
		}
		defer tx2.Terminate()
		res, err = waitResponse(tx2)
		if err != nil {
			return err
		}
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("register status %d", res.StatusCode)
	}
	return nil
}

func waitResponse(tx sip.ClientTransaction) (*sip.Response, error) {
	select {
	case <-tx.Done():
		return nil, fmt.Errorf("transaction done: %v", tx.Err())
	case res := <-tx.Responses():
		return res, nil
	}
}

func mustAtoi(s string) int {
	var n int
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n
}
