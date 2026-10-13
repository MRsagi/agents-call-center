package sip

import (
	"context"
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

const maxAIFFConvertBytes = 8 << 20 // 8 MiB

// audioToUlaw converts WAV/AIFF/PCM/raw input to 8 kHz G.711 μ-law payload.
func audioToUlaw(data []byte) ([]byte, error) {
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		if ulaw, ok := wavToUlaw(data); ok {
			return ulaw, nil
		}
		return nil, fmt.Errorf("unsupported or invalid WAV (SIP Play needs 8 kHz PCM or μ-law WAV)")
	}
	if len(data) >= 12 && string(data[0:4]) == "FORM" {
		formType := string(data[8:12])
		if formType == "AIFF" || formType == "AIFC" {
			if len(data) > maxAIFFConvertBytes {
				return nil, fmt.Errorf("AIFF too large (%d bytes); export 8 kHz μ-law WAV for SIP Play", len(data))
			}
			if ulaw, ok := aiffToUlaw(data); ok {
				return ulaw, nil
			}
			return nil, fmt.Errorf("unsupported or invalid AIFF (need uncompressed PCM; export WAV if needed)")
		}
		return nil, fmt.Errorf("unsupported audio container %q (SIP Play needs WAV or AIFF PCM)", formType)
	}
	if len(data) == 0 {
		return comfortNoiseSamples(8000), nil // 1s silence
	}
	// Non-container payload (e.g. fake synth): short comfort noise proportional to size.
	ms := len(data) * 20
	if ms < 500 {
		ms = 500
	}
	if ms > 30000 {
		ms = 30000
	}
	return comfortNoiseSamples(ms * 8 / 1000), nil
}

func aiffToUlaw(aiff []byte) ([]byte, bool) {
	var (
		commFound bool
		ssndFound bool
		channels  uint16
		bits      uint16
		sampleRate float64
		sound     []byte
		ssndOff   uint32
	)
	off := 12
	for off+8 <= len(aiff) {
		id := string(aiff[off : off+4])
		size := int(binary.BigEndian.Uint32(aiff[off+4 : off+8]))
		off += 8
		if size&1 != 0 {
			size++
		}
		if off+size > len(aiff) {
			break
		}
		chunk := aiff[off : off+size]
		switch id {
		case "COMM":
			if len(chunk) < 18 {
				return nil, false
			}
			channels = binary.BigEndian.Uint16(chunk[0:2])
			bits = binary.BigEndian.Uint16(chunk[6:8])
			sampleRate = readIEEE754Extended(chunk[8:18])
			commFound = true
		case "SSND":
			if len(chunk) < 8 {
				return nil, false
			}
			ssndOff = binary.BigEndian.Uint32(chunk[0:4])
			_ = binary.BigEndian.Uint32(chunk[4:8]) // blockSize
			if int(ssndOff)+8 <= len(chunk) {
				sound = append([]byte(nil), chunk[8+ssndOff:]...)
			}
			ssndFound = true
		}
		off += size
	}
	if !commFound || !ssndFound || channels == 0 || bits != 16 || len(sound) < 2 {
		return nil, false
	}
	rate := int(sampleRate)
	if rate <= 0 {
		rate = rtpClockRate
	}
	pcm := sound
	if channels > 1 {
		pcm = downmixPCM16BE(pcm, int(channels))
	}
	if rate != rtpClockRate {
		pcm = resampleLinearPCM16BE(pcm, rate, rtpClockRate)
	}
	return pcm16BEToUlaw(pcm), true
}

func readIEEE754Extended(b []byte) float64 {
	if len(b) < 10 {
		return 0
	}
	exp := int(b[0]&0x7F)*256 + int(b[1]) - 16383 - 63
	var mant uint64
	for i := 2; i < 10; i++ {
		mant = mant<<8 | uint64(b[i])
	}
	if mant == 0 {
		return 0
	}
	f := float64(mant) / float64(uint64(1)<<63)
	return f * float64(int64(1)<<exp)
}

func downmixPCM16BE(in []byte, ch int) []byte {
	if ch <= 1 {
		return in
	}
	samples := len(in) / (2 * ch)
	out := make([]byte, samples*2)
	for i := 0; i < samples; i++ {
		var sum int32
		for c := 0; c < ch; c++ {
			off := (i*ch + c) * 2
			sum += int32(int16(binary.BigEndian.Uint16(in[off : off+2])))
		}
		binary.BigEndian.PutUint16(out[i*2:i*2+2], uint16(int16(sum/int32(ch))))
	}
	return out
}

func resampleLinearPCM16BE(pcm []byte, fromRate, toRate int) []byte {
	if fromRate == toRate || len(pcm) < 2 {
		return pcm
	}
	le := make([]byte, len(pcm))
	for i := 0; i+1 < len(pcm); i += 2 {
		binary.LittleEndian.PutUint16(le[i:i+2], binary.BigEndian.Uint16(pcm[i:i+2]))
	}
	outLE := resampleLinear(le, fromRate, toRate)
	out := make([]byte, len(outLE))
	for i := 0; i+1 < len(outLE); i += 2 {
		binary.BigEndian.PutUint16(out[i:i+2], binary.LittleEndian.Uint16(outLE[i:i+2]))
	}
	return out
}

func pcm16BEToUlaw(pcm []byte) []byte {
	out := make([]byte, len(pcm)/2)
	for i := 0; i < len(out); i++ {
		s := int16(binary.BigEndian.Uint16(pcm[i*2 : i*2+2]))
		out[i] = linearToUlaw(s)
	}
	return out
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

	rxCancel context.CancelFunc
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
	if r.rxCancel != nil {
		r.rxCancel()
		r.rxCancel = nil
	}
	if r.conn != nil {
		_ = r.conn.Close()
	}
}

// startRXPCM tees inbound RTP μ-law payloads to sink as 8 kHz mono PCM16 LE.
// RX lifetime is tied to the RTP session (until close()), not ring/answer timeouts.
func (r *rtpSender) startRXPCM(sink func([]byte)) {
	if r.conn == nil || sink == nil {
		return
	}
	if r.rxCancel != nil {
		r.rxCancel()
	}
	rxCtx, cancel := context.WithCancel(context.Background())
	r.rxCancel = cancel
	go r.rxLoop(rxCtx, sink)
}

func (r *rtpSender) rxLoop(ctx context.Context, sink func([]byte)) {
	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = r.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := r.conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		if n <= 12 {
			continue
		}
		pt := buf[1] & 0x7F
		if pt != rtpPTPCMU {
			continue
		}
		payload := buf[12:n]
		if len(payload) == 0 {
			continue
		}
		sink(ulawToPCM16LE(payload))
	}
}

func ulawToPCM16LE(ulaw []byte) []byte {
	out := make([]byte, len(ulaw)*2)
	for i, b := range ulaw {
		s := ulawToLinear(b)
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(s))
	}
	return out
}

func ulawToLinear(u byte) int16 {
	u = ^u
	sign := int16(1)
	if u&0x80 != 0 {
		sign = -1
		u &= 0x7F
	}
	exp := int(u>>4) & 0x07
	mant := int(u & 0x0F)
	sample := ((mant << 3) + 0x84) << exp
	return sign * int16(sample-0x84)
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
