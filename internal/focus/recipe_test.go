package focus_test

import (
	"testing"

	"github.com/MRsagi/agents-call-center/internal/focus"
)

func TestKeysForDigits(t *testing.T) {
	keys := focus.KeysForDigits("12")
	if len(keys) != 3 || keys[0] != "1" || keys[1] != "2" || keys[2] != "Enter" {
		t.Fatalf("%v", keys)
	}
	if focus.KeysForDigits("") != nil {
		t.Fatal("empty digits should yield no keys")
	}
}
