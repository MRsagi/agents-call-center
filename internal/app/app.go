package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/domain"
	"github.com/MRsagi/agents-call-center/internal/focus"
	"github.com/MRsagi/agents-call-center/internal/line"
	sipline "github.com/MRsagi/agents-call-center/internal/line/sip"
	"github.com/MRsagi/agents-call-center/internal/phonemic"
	"github.com/MRsagi/agents-call-center/internal/runner"
	"github.com/MRsagi/agents-call-center/internal/synth"
	"github.com/MRsagi/agents-call-center/internal/tray"
)

type App struct {
	Config    config.Config
	Queue     *domain.Queue
	Runner    *runner.Runner
	Voicemail domain.VoicemailStore
	Line      line.Line
	Synth     synth.Synthesizer
	Focus     focus.Focuser
	PhoneMic  phonemic.PhoneMic

	mu         sync.Mutex
	runCancel  context.CancelFunc
	runnerDone chan struct{}
}

type Adapters struct {
	Line  line.Line
	Synth synth.Synthesizer
	Focus focus.Focuser
}

func New(cfg config.Config) (*App, error) {
	return NewWithAdapters(cfg, Adapters{})
}

func NewWithAdapters(cfg config.Config, ad Adapters) (*App, error) {
	def := config.Default()
	if cfg.HTTPBind == "" {
		cfg.HTTPBind = def.HTTPBind
	}
	if cfg.DataDir == "" {
		cfg.DataDir = def.DataDir
	}
	if cfg.RingTimeout == 0 {
		cfg.RingTimeout = def.RingTimeout
	}
	if cfg.LineRecoverWait == 0 {
		cfg.LineRecoverWait = def.LineRecoverWait
	}
	if cfg.FeatureCode == "" {
		cfg.FeatureCode = def.FeatureCode
	}
	if cfg.LineBackend == "" {
		cfg.LineBackend = def.LineBackend
	}
	if cfg.SynthBackend == "" {
		cfg.SynthBackend = def.SynthBackend
	}
	if cfg.FocusBackend == "" {
		cfg.FocusBackend = def.FocusBackend
	}
	if cfg.PhoneMicDevice == "" {
		cfg.PhoneMicDevice = def.PhoneMicDevice
	}
	if cfg.LineBackend == config.LineBackendSIP {
		if cfg.SIPBind == "" {
			cfg.SIPBind = def.SIPBind
		}
		if cfg.SIPPassword == "" {
			cfg.SIPPassword = def.SIPPassword
		}
		if cfg.SIPRealm == "" {
			cfg.SIPRealm = def.SIPRealm
		}
		if cfg.SIPExtension == "" {
			cfg.SIPExtension = def.SIPExtension
		}
		if cfg.SIPUser == "" {
			cfg.SIPUser = cfg.SIPExtension
		}
	}

	ln := ad.Line
	if ln == nil {
		switch cfg.LineBackend {
		case config.LineBackendFake:
			ln = line.NewFake(line.FakeScript{})
		case config.LineBackendVirtual:
			vc := line.VirtualControlFromEnv()
			if cfg.VirtualControlBind != "" {
				vc.ControlAddr = cfg.VirtualControlBind
			}
			vln, verr := line.NewVirtual(vc)
			if verr != nil {
				return nil, verr
			}
			ln = vln
		case config.LineBackendSIP:
			sln, serr := sipline.New(sipline.Config{
				Bind:       cfg.SIPBind,
				User:       cfg.SIPUser,
				Password:   cfg.SIPPassword,
				Realm:      cfg.SIPRealm,
				Extension:  cfg.SIPExtension,
				PublicHost: cfg.SIPPublicHost,
			})
			if serr != nil {
				return nil, serr
			}
			ln = sln
		default:
			return nil, fmt.Errorf("unknown line backend %q", cfg.LineBackend)
		}
	}
	syn := ad.Synth
	if syn == nil {
		switch cfg.SynthBackend {
		case config.SynthBackendFake:
			syn = &synth.Fake{}
		case config.SynthBackendOS:
			syn = synth.NewOS()
		default:
			return nil, fmt.Errorf("unknown synth backend %q", cfg.SynthBackend)
		}
	}
	foc := ad.Focus
	if foc == nil {
		switch cfg.FocusBackend {
		case config.FocusBackendFake:
			foc = &focus.Fake{}
		case config.FocusBackendOS:
			foc = focus.NewOS()
		default:
			return nil, fmt.Errorf("unknown focus backend %q", cfg.FocusBackend)
		}
	}

	var pm phonemic.PhoneMic
	if cfg.PhoneMicEnabled {
		var perr error
		pm, perr = phonemic.New(phonemic.Config{Device: cfg.PhoneMicDevice})
		if perr != nil {
			return nil, perr
		}
		if sln, ok := ln.(*sipline.Line); ok {
			sln.SetRXPCMHook(pm.WritePCM)
		} else {
			log.Printf("phone mic: enabled but line backend %q has no RX PCM path (SIP only)", cfg.LineBackend)
		}
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	vmPath := filepath.Join(cfg.DataDir, "voicemail.jsonl")
	vm, err := domain.OpenVoicemailStore(vmPath)
	if err != nil {
		return nil, err
	}

	q := domain.NewQueue()
	r := runner.New(runner.Deps{
		Queue:           q,
		Line:            ln,
		Synth:           syn,
		Focus:           foc,
		Voicemail:       vm,
		Dialplan:        domain.NewDialplan(cfg.FeatureCode, cfg.IdleDictationCode),
		RingTimeout:     cfg.RingTimeout,
		LineRecoverWait: cfg.LineRecoverWait,
	})

	return &App{
		Config:    cfg,
		Queue:     q,
		Runner:    r,
		Voicemail: vm,
		Line:      ln,
		Synth:     syn,
		Focus:     foc,
		PhoneMic:  pm,
	}, nil
}

func (a *App) Start(parent context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runCancel != nil {
		return errors.New("app already started")
	}
	if a.PhoneMic != nil {
		if err := a.PhoneMic.Start(); err != nil {
			return fmt.Errorf("phone mic: %w", err)
		}
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	a.runCancel = cancel
	a.runnerDone = done
	go func() {
		defer close(done)
		a.Runner.Start(ctx)
	}()
	return nil
}

func (a *App) Stop() {
	a.mu.Lock()
	cancel := a.runCancel
	done := a.runnerDone
	a.runCancel = nil
	a.runnerDone = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	if a.PhoneMic != nil {
		a.PhoneMic.Stop()
	}
}

func (a *App) LineUp() bool {
	if rl, ok := a.Line.(line.ReadyLine); ok {
		return rl.Ready()
	}
	return true
}

// StatusSnapshot implements tray.StatusProvider for the status tray.
func (a *App) StatusSnapshot() tray.StatusSnapshot {
	vmCount := 0
	if a.Voicemail != nil {
		if n, err := a.Voicemail.Count(); err == nil {
			vmCount = n
		}
	}
	return tray.StatusSnapshot{
		QueueDepth:      a.Queue.Len(),
		VoicemailCount:  vmCount,
		LineUp:          a.LineUp(),
		LastCallSummary: a.Queue.LastFinishedSummary(),
	}
}
