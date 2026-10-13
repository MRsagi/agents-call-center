package sip

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	rtpClockRate = 8000
	rtpPTPCMU    = 0
)

type rtpEndpoint struct {
	addr *net.UDPAddr
}

func parseAudioSDP(body []byte) (rtpEndpoint, error) {
	var ep rtpEndpoint
	if len(body) == 0 {
		return ep, fmt.Errorf("empty SDP")
	}
	lines := splitLines(string(body))
	var audioPort int
	var connIP string
	for _, ln := range lines {
		if len(ln) >= 2 && ln[0] == 'm' && ln[1] == '=' {
			// m=audio 49170 RTP/AVP 0
			var media string
			fmt.Sscanf(ln, "m=%s %d", &media, &audioPort)
			if media != "audio" {
				continue
			}
		}
		if len(ln) >= 2 && ln[0] == 'c' && ln[1] == '=' {
			var netw string
			fmt.Sscanf(ln, "c=%s IP4 %s", &netw, &connIP)
		}
	}
	if audioPort == 0 || connIP == "" {
		return ep, fmt.Errorf("SDP missing audio connection")
	}
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", connIP, audioPort))
	if err != nil {
		return ep, err
	}
	ep.addr = addr
	return ep, nil
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			ln := s[start:i]
			if len(ln) > 0 && ln[len(ln)-1] == '\r' {
				ln = ln[:len(ln)-1]
			}
			out = append(out, ln)
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func buildOfferSDP(publicHost string, rtpPort int) string {
	return fmt.Sprintf(
		"v=0\r\n"+
			"o=acc 0 0 IN IP4 %s\r\n"+
			"s=acc\r\n"+
			"c=IN IP4 %s\r\n"+
			"t=0 0\r\n"+
			"m=audio %d RTP/AVP 0 101\r\n"+
			"a=rtpmap:0 PCMU/8000\r\n"+
			"a=rtpmap:101 telephone-event/8000\r\n"+
			"a=fmtp:101 0-15\r\n"+
			"a=sendrecv\r\n",
		publicHost, publicHost, rtpPort,
	)
}

// audioToUlaw converts WAV/PCM/raw input to 8 kHz G.711 μ-law payload.
func audioToUlaw(data []byte) []byte {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		if ulaw, ok := wavToUlaw(data); ok {
			return ulaw
		}
	}
	if len(data) == 0 {
		return comfortNoiseSamples(8000) // 1s silence
	}
	// Non-WAV (e.g. fake synth): short comfort noise proportional to payload size.
	ms := len(data) * 20
	if ms < 500 {
		ms = 500
	}
	if ms > 30000 {
		ms = 30000
	}
	return comfortNoiseSamples(ms * 8 / 1000)
}

func wavToUlaw(wav []byte) ([]byte, bool) {
	var (
		fmtFound   bool
		dataFound  bool
		audioFmt   uint16
		channels   uint16
		sampleRate uint32
		bits       uint16
		data       []byte
	)
	off := 12
	for off+8 <= len(wav) {
		id := string(wav[off : off+4])
		size := int(binary.LittleEndian.Uint32(wav[off+4 : off+8]))
		off += 8
		if off+size > len(wav) {
			break
		}
		chunk := wav[off : off+size]
		switch id {
		case "fmt ":
			if len(chunk) < 16 {
				return nil, false
			}
			audioFmt = binary.LittleEndian.Uint16(chunk[0:2])
			channels = binary.LittleEndian.Uint16(chunk[2:4])
			sampleRate = binary.LittleEndian.Uint32(chunk[4:8])
			bits = binary.LittleEndian.Uint16(chunk[14:16])
			fmtFound = true
		case "data":
			data = append([]byte(nil), chunk...)
			dataFound = true
		}
		off += size
	}
	if !fmtFound || !dataFound || channels == 0 {
		return nil, false
	}
	switch audioFmt {
	case 7: // μ-law WAV
		if channels > 1 {
			return downmixUlaw(data, int(channels)), true
		}
		return data, true
	case 1: // PCM
		if bits != 16 {
			return nil, false
		}
		pcm := data
		if channels > 1 {
			pcm = downmixPCM16(pcm, int(channels))
		}
		if sampleRate != rtpClockRate {
			pcm = resampleLinear(pcm, int(sampleRate), rtpClockRate)
		}
		return pcm16ToUlaw(pcm), true
	default:
		return nil, false
	}
}

func downmixUlaw(in []byte, ch int) []byte {
	if ch <= 1 {
		return in
	}
	out := make([]byte, 0, len(in)/ch)
	for i := 0; i+ch <= len(in); i += ch {
		out = append(out, in[i])
	}
	return out
}

func downmixPCM16(in []byte, ch int) []byte {
	if ch <= 1 {
		return in
	}
	samples := len(in) / (2 * ch)
	out := make([]byte, samples*2)
	for i := 0; i < samples; i++ {
		var sum int32
		for c := 0; c < ch; c++ {
			off := (i*ch + c) * 2
			sum += int32(int16(binary.LittleEndian.Uint16(in[off : off+2])))
		}
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(int16(sum/int32(ch))))
	}
	return out
}

func resampleLinear(pcm []byte, fromRate, toRate int) []byte {
	if fromRate == toRate || len(pcm) < 2 {
		return pcm
	}
	inSamples := len(pcm) / 2
	outSamples := inSamples * toRate / fromRate
	if outSamples < 1 {
		outSamples = 1
	}
	out := make([]byte, outSamples*2)
	for i := 0; i < outSamples; i++ {
		src := float64(i) * float64(fromRate) / float64(toRate)
		j := int(src)
		if j >= inSamples-1 {
			j = inSamples - 2
		}
		frac := src - float64(j)
		s0 := float64(int16(binary.LittleEndian.Uint16(pcm[j*2 : j*2+2])))
		s1 := float64(int16(binary.LittleEndian.Uint16(pcm[(j+1)*2 : (j+1)*2+2])))
		v := int16(s0 + (s1-s0)*frac)
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(v))
	}
	return out
}

func pcm16ToUlaw(pcm []byte) []byte {
	out := make([]byte, len(pcm)/2)
	for i := 0; i < len(out); i++ {
		s := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		out[i] = linearToUlaw(s)
	}
	return out
}

func linearToUlaw(sample int16) byte {
	const bias = 0x84
	sign := byte(0)
	if sample < 0 {
		sign = 0x80
		sample = -sample
	}
	if sample > 0x7FFF {
		sample = 0x7FFF
	}
	sample += bias
	exp := 7
	for exp > 0 && (sample&0x4000) == 0 {
		sample <<= 1
		exp--
	}
	mantissa := (sample >> 10) & 0x0F
	return ^(sign | byte(exp<<4) | byte(mantissa))
}

func comfortNoiseSamples(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = 0xFF // μ-law silence
	}
	return out
}

type rtpSender struct {
	conn   *net.UDPConn
	remote *net.UDPAddr
	seq    uint16
	ts     uint32
	ssrc   uint32
	mu     sync.Mutex
}

func newRTPSender(bindHost string) (*rtpSender, error) {
	laddr, err := net.ResolveUDPAddr("udp", bindHost+":0")
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, err
	}
	return &rtpSender{
		conn: conn,
		ssrc: uint32(time.Now().UnixNano()),
	}, nil
}

func (r *rtpSender) localPort() int {
	if r.conn == nil {
		return 0
	}
	return r.conn.LocalAddr().(*net.UDPAddr).Port
}

func (r *rtpSender) setRemote(ep rtpEndpoint) {
	r.mu.Lock()
	r.remote = ep.addr
	r.mu.Unlock()
}

func (r *rtpSender) play(ctx <-chan struct{}, payload []byte) error {
	if r.conn == nil || r.remote == nil {
		return fmt.Errorf("rtp not configured")
	}
	const frameSamples = 160 // 20 ms @ 8 kHz
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	pos := 0
	for {
		select {
		case <-ctx:
			return nil
		case <-ticker.C:
			var frame []byte
			if pos < len(payload) {
				end := pos + frameSamples
				if end > len(payload) {
					end = len(payload)
				}
				frame = payload[pos:end]
				pos = end
			} else {
				frame = comfortNoiseSamples(frameSamples)
			}
			if len(frame) < frameSamples {
				pad := make([]byte, frameSamples)
				copy(pad, frame)
				for i := len(frame); i < frameSamples; i++ {
					pad[i] = 0xFF
				}
				frame = pad
			}
			if err := r.sendFrame(frame); err != nil {
				return err
			}
		}
	}
}

func (r *rtpSender) sendFrame(payload []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.remote == nil {
		return fmt.Errorf("no rtp remote")
	}
	pkt := make([]byte, 12+len(payload))
	pkt[0] = 0x80
	pkt[1] = rtpPTPCMU
	binary.BigEndian.PutUint16(pkt[2:4], r.seq)
	r.seq++
	binary.BigEndian.PutUint32(pkt[4:8], r.ts)
	r.ts += uint32(len(payload))
	binary.BigEndian.PutUint32(pkt[8:12], r.ssrc)
	copy(pkt[12:], payload)
	_, err := r.conn.WriteToUDP(pkt, r.remote)
	return err
}

func (r *rtpSender) close() {
	if r.conn != nil {
		_ = r.conn.Close()
	}
}

func parseDTMFSignal(body, contentType string) (rune, bool) {
	_ = contentType
	for _, ln := range splitLines(body) {
		if len(ln) >= 7 && ln[:7] == "Signal=" {
			d := strings.TrimSpace(ln[7:])
			if len(d) >= 1 {
				return rune(d[0]), true
			}
		}
	}
	if len(body) == 1 {
		switch body[0] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '*', '#':
			return rune(body[0]), true
		}
	}
	return 0, false
}
