package tray

import "testing"

func TestFormatTooltip(t *testing.T) {
	tests := []struct {
		name string
		in   StatusSnapshot
		want string
	}{
		{
			name: "line up",
			in:   StatusSnapshot{QueueDepth: 3, VoicemailCount: 2, LineUp: true},
			want: "acc · queue 3 · vm 2 · line up",
		},
		{
			name: "line down",
			in:   StatusSnapshot{QueueDepth: 0, VoicemailCount: 0, LineUp: false},
			want: "acc · queue 0 · vm 0 · line down",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatTooltip(tt.in); got != tt.want {
				t.Fatalf("FormatTooltip() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdateTooltip(t *testing.T) {
	title, tooltip, status, vm := UpdateTooltip(StatusSnapshot{
		QueueDepth:      1,
		VoicemailCount:  4,
		LineUp:          true,
		LastCallSummary: "abc · completed",
	})
	wantTitle := "acc · queue 1 · vm 4 · line up"
	if title != wantTitle || tooltip != wantTitle {
		t.Fatalf("title/tooltip = %q / %q", title, tooltip)
	}
	if status != "abc · completed" {
		t.Fatalf("status menu = %q", status)
	}
	if vm != "Voicemails: 4" {
		t.Fatalf("vm menu = %q", vm)
	}

	_, _, status, _ = UpdateTooltip(StatusSnapshot{})
	if status != "No recent calls" {
		t.Fatalf("empty summary = %q", status)
	}
}

type fakeProvider struct {
	called *bool
}

func (f fakeProvider) StatusSnapshot() StatusSnapshot {
	*f.called = true
	return StatusSnapshot{}
}

func TestRunDisabledByEnv(t *testing.T) {
	t.Setenv("ACC_TRAY", "0")
	called := false
	Run(t.Context(), fakeProvider{called: &called})
	if called {
		t.Fatal("provider polled when ACC_TRAY=0")
	}
}
