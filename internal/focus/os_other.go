//go:build !darwin && !windows

package focus

import (
	"context"
	"errors"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

type unsupportedFocuser struct{}

func newOSImpl() Focuser {
	return &unsupportedFocuser{}
}

func (u *unsupportedFocuser) Focus(ctx context.Context, target domain.FocusTarget) error {
	return errors.New("focuser not supported on this OS")
}

func (u *unsupportedFocuser) Type(ctx context.Context, keys []string) error {
	return errors.New("focuser not supported on this OS")
}
