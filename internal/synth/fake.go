package synth

import (
	"context"
	"errors"
)

type Fake struct {
	Fail bool
}

func (f *Fake) Synthesize(_ context.Context, prompt string) ([]byte, error) {
	if f.Fail {
		return nil, errors.New("synthesize failed")
	}
	return []byte(prompt), nil
}
