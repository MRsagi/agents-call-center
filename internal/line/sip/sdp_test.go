package sip

import "testing"

func TestParseAudioSDP_SessionAndMediaConn(t *testing.T) {
	body := []byte("v=0\r\n" +
		"o=- 1 1 IN IP4 192.168.10.75\r\n" +
		"s=-\r\n" +
		"c=IN IP4 192.168.10.75\r\n" +
		"t=0 0\r\n" +
		"m=audio 16456 RTP/AVP 0 101\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n")
	ep, err := parseAudioSDP(body)
	if err != nil {
		t.Fatal(err)
	}
	if ep.addr.String() != "192.168.10.75:16456" {
		t.Fatalf("addr=%s", ep.addr)
	}
}

func TestParseAudioSDP_MediaLevelOverridesSession(t *testing.T) {
	body := []byte("v=0\r\n" +
		"c=IN IP4 10.0.0.1\r\n" +
		"m=audio 20000 RTP/AVP 0\r\n" +
		"c=IN IP4 192.168.10.75\r\n")
	ep, err := parseAudioSDP(body)
	if err != nil {
		t.Fatal(err)
	}
	if ep.addr.String() != "192.168.10.75:20000" {
		t.Fatalf("addr=%s", ep.addr)
	}
}
