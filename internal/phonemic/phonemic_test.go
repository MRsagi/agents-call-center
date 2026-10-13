package phonemic_test

import (
	"testing"

	"github.com/MRsagi/agents-call-center/internal/phonemic"
)

func TestFake_RecordsPCM(t *testing.T) {
	var f phonemic.Fake
	if err := f.Start(); err != nil {
		t.Fatal(err)
	}
	if !f.Started() {
		t.Fatal("not started")
	}
	f.WritePCM([]byte{1, 2, 3, 4})
	f.Stop()
	if f.Started() {
		t.Fatal("still started after Stop")
	}
	got := f.PCM()
	if len(got) != 4 {
		t.Fatalf("pcm len=%d want 4", len(got))
	}
}

func TestNew_NonDarwinStub(t *testing.T) {
	pm, err := phonemic.New(phonemic.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	pm.WritePCM([]byte{0})
	pm.Stop()
}
