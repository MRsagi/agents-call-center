package domain_test

import (
	"path/filepath"
	"testing"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestVoicemail_LeaveOldestMarkHeard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "voicemail.jsonl")
	store, err := domain.OpenVoicemailStore(path)
	if err != nil {
		t.Fatal(err)
	}
	c1, _ := domain.NewCall("first", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	c2, _ := domain.NewCall("second", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	if err := store.Leave(c1); err != nil {
		t.Fatal(err)
	}
	if err := store.Leave(c2); err != nil {
		t.Fatal(err)
	}
	n, err := store.Count()
	if err != nil || n != 2 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	oldest, err := store.OldestUnheard()
	if err != nil || oldest.CallID != c1.ID {
		t.Fatalf("oldest=%#v err=%v", oldest, err)
	}
	if err := store.MarkHeard(c1.ID); err != nil {
		t.Fatal(err)
	}
	n, _ = store.Count()
	if n != 1 {
		t.Fatalf("count after heard=%d", n)
	}
	oldest, err = store.OldestUnheard()
	if err != nil || oldest.CallID != c2.ID {
		t.Fatalf("oldest after=%#v", oldest)
	}
}

func TestVoicemail_PersistsAcrossOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "voicemail.jsonl")
	store, _ := domain.OpenVoicemailStore(path)
	c, _ := domain.NewCall("persist", domain.ModeDTMF, domain.FocusTarget{}, 1)
	_ = store.Leave(c)
	store2, err := domain.OpenVoicemailStore(path)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := store2.Count()
	if n != 1 {
		t.Fatalf("count=%d", n)
	}
}
