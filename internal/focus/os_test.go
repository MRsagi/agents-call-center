package focus_test

import (
	"runtime"
	"testing"

	"github.com/MRsagi/agents-call-center/internal/focus"
)

func TestNewOS_ImplementsFocuser(t *testing.T) {
	f := focus.NewOS()
	if f == nil {
		t.Fatal("NewOS returned nil")
	}
	var _ focus.Focuser = f
}

func TestNewOS_SupportedPlatformsNonNil(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "windows":
	default:
		t.Skip("OS focuser platform check runs on darwin and windows only")
	}
	if focus.NewOS() == nil {
		t.Fatal("NewOS returned nil")
	}
}
