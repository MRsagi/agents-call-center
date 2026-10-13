package synth

import "context"

type Synthesizer interface {
	Synthesize(ctx context.Context, prompt string) ([]byte, error)
}
