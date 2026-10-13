package sip

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestUlawPCMRoundtrip(t *testing.T) {
	samples := []int16{-1000, 0, 1000, 8000, -8000}
	for _, s := range samples {
		u := linearToUlaw(s)
		got := ulawToLinear(u)
		diff := got - s
		if diff < 0 {
			diff = -diff
		}
		if diff > 512 {
			t.Fatalf("sample %d ulaw roundtrip diff %d", s, diff)
		}
	}
}

func TestStartRXPCMSurvivesRingContextCancel(t *testing.T) {
	r, err := newRTPSender("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.close() })

	remote, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+itoa(r.localPort()))
	if err != nil {
		t.Fatal(err)
	}
	r.setRemote(rtpEndpoint{addr: remote})

	var got atomic.Int32
	r.startRXPCM(func([]byte) { got.Add(1) })

	ringCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = ringCtx

	conn, err := net.DialUDP("udp", nil, r.conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payload := []byte{linearToUlaw(1000)}
	pkt := make([]byte, 12+len(payload))
	pkt[0] = 0x80
	pkt[1] = rtpPTPCMU
	binary.BigEndian.PutUint16(pkt[2:4], 1)
	binary.BigEndian.PutUint32(pkt[8:12], 1)
	copy(pkt[12:], payload)

	deadline := time.Now().Add(2 * time.Second)
	for got.Load() == 0 && time.Now().Before(deadline) {
		_, _ = conn.Write(pkt)
		time.Sleep(50 * time.Millisecond)
	}
	if got.Load() == 0 {
		t.Fatal("RX tee stopped after ring context cancel; want call-scoped lifetime")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
