package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusQueued     Status = "queued"
	StatusRinging    Status = "ringing"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusVoicemail  Status = "voicemail"
	StatusFailed     Status = "failed"
)

type Mode string

const (
	ModeNotify Mode = "notify"
	ModeDTMF   Mode = "dtmf"
	ModeSpeech Mode = "speech"
)

type Delivery string

const (
	DeliveryOK          Delivery = "ok"
	DeliveryFocusMissed Delivery = "focus_missed"
)

type FocusTarget struct {
	App           string `json:"app"`
	TitleContains string `json:"title_contains,omitempty"`
	ChatID        string `json:"chat_id,omitempty"`
}

type Call struct {
	ID          string      `json:"id"`
	Prompt      string      `json:"prompt"`
	Mode        Mode        `json:"mode"`
	MaxDigits   int         `json:"max_digits"`
	FocusTarget FocusTarget `json:"focus_target"`
	CreatedAt   time.Time   `json:"created_at"`
	Status      Status      `json:"status"`
	Digits      string      `json:"digits,omitempty"`
	Delivery    Delivery    `json:"delivery,omitempty"`
	Error       string      `json:"error,omitempty"`
}

func NewCall(prompt string, mode Mode, ft FocusTarget, maxDigits int) (*Call, error) {
	if prompt == "" {
		return nil, errors.New("prompt required")
	}
	switch mode {
	case ModeDTMF:
		// ok — only implemented mode in v1
	case ModeNotify, ModeSpeech:
		return nil, errors.New("mode not implemented in v1: " + string(mode))
	default:
		return nil, errors.New("unknown mode")
	}
	if maxDigits <= 0 {
		maxDigits = 1
	}
	return &Call{
		ID:          uuid.NewString(),
		Prompt:      prompt,
		Mode:        mode,
		MaxDigits:   maxDigits,
		FocusTarget: ft,
		CreatedAt:   time.Now().UTC(),
		Status:      StatusQueued,
	}, nil
}
