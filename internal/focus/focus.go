package focus

import (
	"context"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

type Focuser interface {
	Focus(ctx context.Context, target domain.FocusTarget) error
	Type(ctx context.Context, keys []string) error
}
