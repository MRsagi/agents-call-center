package sip

import "testing"

func TestUlawPCMRoundtrip(t *testing.T) {
	samples := []int16{-1000, 0, 1000, 8000, -8000}
	for _, s := range samples {
		u := linearToUlaw(s)
		got := ulawToLinear(u)
		diff := got - s
		if diff < 0 {
			diff = -diff
		}
		if diff > 512 {
			t.Fatalf("sample %d ulaw roundtrip diff %d", s, diff)
		}
	}
}
