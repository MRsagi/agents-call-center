package sip

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/MRsagi/agents-call-center/internal/line"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/icholy/digest"
)

// Config configures the embedded SIP registrar/UA for ATA devices (e.g. PAP2T).
type Config struct {
	Bind        string
	User        string
	Password    string
	Realm       string
	Extension   string
	PublicHost  string
	Logf        func(format string, args ...any)
}

// Line implements line.Line and line.ReadyLine for PAP2T-style registration.
type Line struct {
	cfg Config

	ua     *sipgo.UserAgent
	srv    *sipgo.Server
	client *sipgo.Client
	dialog *sipgo.DialogClientCache

	mu            sync.RWMutex
	contact       sip.Uri
	contactDest   string
	registered    bool
	nonce         string
	listenAddr    string
	publicHost    string

	idleHandler func(digits string)
	idleAgg     *line.IdleDigitAggregator

	callMu      sync.Mutex
	dialogSess  *sipgo.DialogClientSession
	rtp         *rtpSender
	answerErr   error
	answered    bool
	hangupCh    chan struct{}
	digitCh     chan rune
	collecting  bool
	collectMax  int

	rxPCM func([]byte)
}

func New(cfg Config) (*Line, error) {
	defBind := "0.0.0.0:5060"
	if cfg.Bind == "" {
		cfg.Bind = defBind
	}
	if cfg.Realm == "" {
		cfg.Realm = "agents-call-center"
	}
	if cfg.Extension == "" {
		cfg.Extension = "100"
	}
	if cfg.User == "" {
		cfg.User = cfg.Extension
	}
	if cfg.Logf == nil {
		cfg.Logf = func(format string, args ...any) { log.Printf(format, args...) }
	}
	publicHost := cfg.PublicHost
	if publicHost == "" {
		publicHost = detectPublicHost(cfg.Bind)
	}

	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent("agents-call-center"),
		sipgo.WithUserAgentHostname(publicHost),
	)
	if err != nil {
		return nil, err
	}
	srv, err := sipgo.NewServer(ua)
	if err != nil {
		_ = ua.Close()
		return nil, err
	}
	client, err := sipgo.NewClient(ua, sipgo.WithClientHostname(publicHost))
	if err != nil {
		_ = ua.Close()
		return nil, err
	}

	contactHDR := sip.ContactHeader{
		Address: sip.Uri{
			User: "acc",
			Host: publicHost,
		},
	}
	dialog := sipgo.NewDialogClientCache(client, contactHDR)

	l := &Line{
		cfg:        cfg,
		ua:         ua,
		srv:        srv,
		client:     client,
		dialog:     dialog,
		publicHost: publicHost,
		digitCh:    make(chan rune, 32),
		hangupCh:   make(chan struct{}, 1),
	}

	l.srv.OnRegister(l.onRegister)
	l.srv.OnInfo(l.onInfo)
	l.srv.OnBye(l.onBye)

	ready := make(chan struct{}, 1)
	ctx := context.Background()
	ctx = context.WithValue(ctx, sipgo.ListenReadyCtxKey, sipgo.ListenReadyFuncCtxValue(func(network, addr string) {
		l.mu.Lock()
		l.listenAddr = addr
		l.mu.Unlock()
		select {
		case ready <- struct{}{}:
		default:
		}
	}))

	go func() {
		if err := srv.ListenAndServe(ctx, "udp", cfg.Bind); err != nil && !errors.Is(err, context.Canceled) {
			cfg.Logf("sip line listen: %v", err)
		}
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		_ = ua.Close()
		return nil, fmt.Errorf("sip line: timed out waiting for UDP listen on %s", cfg.Bind)
	}

	cfg.Logf("sip line: listening on %s (public %s)", l.listenAddr, publicHost)
	return l, nil
}

func detectPublicHost(bind string) string {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		host = bind
	}
	if host != "" && host != "0.0.0.0" && host != "[::]" {
		return host
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			return ipNet.IP.String()
		}
	}
	return "127.0.0.1"
}

func (l *Line) ListenAddr() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.listenAddr
}

func (l *Line) Ready() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.registered
}

func (l *Line) OnIdleDigits(handler func(digits string)) {
	l.idleHandler = handler
	l.idleAgg = line.NewIdleDigitAggregator(line.DefaultIdleDigitTimeout, handler)
}

func (l *Line) ClearIdleDigits() {
	if l.idleAgg != nil {
		l.idleAgg.Clear()
	}
}

// SetRXPCMHook tees inbound RTP audio (handset → acc) to sink when off-hook.
func (l *Line) SetRXPCMHook(sink func([]byte)) {
	l.callMu.Lock()
	l.rxPCM = sink
	l.callMu.Unlock()
}

func (l *Line) Ring(ctx context.Context) error {
	l.callMu.Lock()
	defer l.callMu.Unlock()

	l.resetCallLocked()

	contact, dest, ok := l.registeredContact()
	if !ok {
		return errors.New("sip line: no ATA registered")
	}

	rtp, err := newRTPSender(l.publicHost)
	if err != nil {
		return err
	}
	l.rtp = rtp
	sdp := buildOfferSDP(l.publicHost, rtp.localPort())

	// WriteInvite builds the dialog and sends INVITE once (DialogUA.Invite
	// already calls session.Invite). Destination must be set before send.
	req := sip.NewRequest(sip.INVITE, contact)
	req.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	req.SetBody([]byte(sdp))
	if dest != "" {
		req.SetDestination(dest)
	}
	invite, err := l.dialog.WriteInvite(ctx, req)
	if err != nil {
		rtp.close()
		l.rtp = nil
		return fmt.Errorf("sip invite send: %w", err)
	}
	l.dialogSess = invite
	return nil
}

func (l *Line) WaitAnswer(ctx context.Context) error {
	l.callMu.Lock()
	sess := l.dialogSess
	l.callMu.Unlock()
	if sess == nil {
		return errors.New("sip line: no active invite")
	}

	err := sess.WaitAnswer(ctx, sipgo.AnswerOptions{})
	if err != nil {
		l.callMu.Lock()
		l.answerErr = err
		l.callMu.Unlock()
		return err
	}
	if err := sess.Ack(ctx); err != nil {
		return err
	}

	if sess.InviteResponse != nil && l.rtp != nil {
		body := sess.InviteResponse.Body()
		l.cfg.Logf("sip line: answer SDP: %q", truncateForLog(body, 500))
		ep, err := parseAudioSDP(body)
		if err != nil {
			l.cfg.Logf("sip line: sdp parse: %v", err)
		} else {
			l.cfg.Logf("sip line: rtp remote %s (local port %d)", ep.addr, l.rtp.localPort())
			l.rtp.setRemote(ep)
		}
	}

	l.callMu.Lock()
	l.answered = true
	sink := l.rxPCM
	rtp := l.rtp
	l.callMu.Unlock()
	if rtp != nil {
		// Keepalive immediately so Focus/TTS setup cannot starve the ATA media timer.
		rtp.startKeepalive()
	}
	if sink != nil && rtp != nil {
		rtp.startRXPCM(sink)
	}
	return nil
}

func (l *Line) Play(ctx context.Context, audio []byte) error {
	l.callMu.Lock()
	rtp := l.rtp
	l.callMu.Unlock()
	if rtp == nil {
		return errors.New("sip line: no rtp session")
	}
	payload, err := audioToUlaw(audio)
	if err != nil {
		l.cfg.Logf("sip line: play audio: %v", err)
		return err
	}
	l.cfg.Logf("sip line: playing %d μ-law samples (~%.1fs) -> %v", len(payload), float64(len(payload))/float64(rtpClockRate), rtp.remoteAddr())
	// Brief silence lets some ATAs open the RX path before speech.
	pre := comfortNoiseSamples(rtpClockRate / 5) // 200 ms
	payload = append(pre, payload...)
	return rtp.play(ctx.Done(), payload)
}

func (l *Line) CollectDigits(ctx context.Context, max int) (string, error) {
	if max <= 0 {
		max = 16
	}
	l.callMu.Lock()
	l.collecting = true
	l.collectMax = max
	l.callMu.Unlock()
	defer func() {
		l.callMu.Lock()
		l.collecting = false
		l.callMu.Unlock()
	}()

	var b strings.Builder
	for b.Len() < max {
		select {
		case <-ctx.Done():
			return b.String(), ctx.Err()
		case <-l.hangupCh:
			if b.Len() == 0 {
				return "", errors.New("sip line: remote hangup")
			}
			return b.String(), nil
		case d := <-l.digitCh:
			b.WriteRune(d)
			if b.Len() >= max {
				return b.String(), nil
			}
		}
	}
	return b.String(), nil
}

func (l *Line) Hangup(ctx context.Context) error {
	l.callMu.Lock()
	sess := l.dialogSess
	rtp := l.rtp
	l.dialogSess = nil
	l.rtp = nil
	l.answered = false
	l.callMu.Unlock()

	if rtp != nil {
		rtp.close()
	}
	if sess != nil {
		if err := sess.Bye(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func (l *Line) Close() error {
	if l.ua != nil {
		return l.ua.Close()
	}
	return nil
}

func (l *Line) registeredContact() (sip.Uri, string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if !l.registered {
		return sip.Uri{}, "", false
	}
	return l.contact, l.contactDest, true
}

func (l *Line) resetCallLocked() {
	l.dialogSess = nil
	if l.rtp != nil {
		l.rtp.close()
		l.rtp = nil
	}
	l.answered = false
	l.answerErr = nil
	select {
	case <-l.hangupCh:
	default:
	}
	for len(l.digitCh) > 0 {
		<-l.digitCh
	}
}

func (l *Line) onRegister(req *sip.Request, tx sip.ServerTransaction) {
	to := req.To()
	if to == nil || to.Address.User != l.cfg.Extension {
		res := sip.NewResponseFromRequest(req, 404, "Not Found", nil)
		_ = tx.Respond(res)
		return
	}

	expires := registerExpires(req)
	if expires == 0 {
		l.mu.Lock()
		l.registered = false
		l.mu.Unlock()
		res := sip.NewResponseFromRequest(req, 200, "OK", nil)
		_ = tx.Respond(res)
		return
	}

	if l.cfg.Password != "" {
		auth := req.GetHeader("Authorization")
		if auth == nil {
			l.mu.Lock()
			if l.nonce == "" {
				l.nonce = randomNonce()
			}
			nonce := l.nonce
			l.mu.Unlock()
			hdr := fmt.Sprintf(`Digest realm="%s", nonce="%s", algorithm=MD5`, l.cfg.Realm, nonce)
			res := sip.NewResponseFromRequest(req, 401, "Unauthorized", nil)
			res.AppendHeader(sip.NewHeader("WWW-Authenticate", hdr))
			_ = tx.Respond(res)
			return
		}
		if !l.verifyRegisterAuth(req, auth.Value()) {
			res := sip.NewResponseFromRequest(req, 403, "Forbidden", nil)
			_ = tx.Respond(res)
			return
		}
	}

	contact := req.Contact()
	if contact == nil {
		res := sip.NewResponseFromRequest(req, 400, "Bad Request", nil)
		_ = tx.Respond(res)
		return
	}

	dest := contact.Address.Host
	if contact.Address.Port != 0 {
		dest = fmt.Sprintf("%s:%d", contact.Address.Host, contact.Address.Port)
	} else {
		dest = net.JoinHostPort(contact.Address.Host, "5060")
	}

	l.mu.Lock()
	l.contact = contact.Address
	l.contactDest = dest
	l.registered = true
	l.mu.Unlock()

	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	_ = tx.Respond(res)
	l.cfg.Logf("sip line: registered contact %s", contact.Address.String())
}

func registerExpires(req *sip.Request) int {
	if h := req.GetHeader("Expires"); h != nil {
		var v int
		fmt.Sscanf(h.Value(), "%d", &v)
		return v
	}
	if c := req.Contact(); c != nil {
		if exp, ok := c.Params.Get("expires"); ok {
			var v int
			fmt.Sscanf(exp, "%d", &v)
			return v
		}
	}
	return 3600
}

func (l *Line) verifyRegisterAuth(req *sip.Request, authHeader string) bool {
	cred, err := digest.ParseCredentials(authHeader)
	if err != nil {
		return false
	}
	if cred.Username != l.cfg.User && cred.Username != l.cfg.Extension {
		return false
	}
	l.mu.RLock()
	nonce := l.nonce
	l.mu.RUnlock()
	chal := &digest.Challenge{
		Realm:     l.cfg.Realm,
		Nonce:     nonce,
		Algorithm: "MD5",
	}
	opts := digest.Options{
		Method:   req.Method.String(),
		URI:      cred.URI,
		Username: cred.Username,
		Password: l.cfg.Password,
	}
	expected, err := digest.Digest(chal, opts)
	if err != nil {
		return false
	}
	return cred.Response == expected.Response
}

func randomNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (l *Line) onInfo(req *sip.Request, tx sip.ServerTransaction) {
	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	_ = tx.Respond(res)

	var signal string
	if h := req.GetHeader("Signal"); h != nil {
		signal = strings.TrimSpace(h.Value())
	}
	body := string(req.Body())
	ct := ""
	if h := req.GetHeader("Content-Type"); h != nil {
		ct = h.Value()
	}
	var digit rune
	var ok bool
	if signal != "" && len(signal) >= 1 {
		digit = rune(signal[0])
		ok = true
	} else {
		digit, ok = parseDTMFSignal(body, ct)
	}
	if !ok {
		return
	}

	l.callMu.Lock()
	collecting := l.collecting
	l.callMu.Unlock()
	if collecting {
		select {
		case l.digitCh <- digit:
		default:
		}
		return
	}
	if l.idleAgg != nil {
		l.idleAgg.Add(string(digit))
	} else if l.idleHandler != nil {
		l.idleHandler(string(digit))
	}
}

func (l *Line) onBye(req *sip.Request, tx sip.ServerTransaction) {
	if err := l.dialog.ReadBye(req, tx); err == nil {
		select {
		case l.hangupCh <- struct{}{}:
		default:
		}
		return
	}
	res := sip.NewResponseFromRequest(req, 481, "Call/Transaction Does Not Exist", nil)
	_ = tx.Respond(res)
}

func truncateForLog(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

var (
	_ line.Line      = (*Line)(nil)
	_ line.ReadyLine = (*Line)(nil)
)
