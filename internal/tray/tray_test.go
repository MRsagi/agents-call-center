package tray

import "testing"

func TestConsoleURL(t *testing.T) {
	if got := ConsoleURL("127.0.0.1:8741"); got != "http://127.0.0.1:8741/console/" {
		t.Fatal(got)
	}
}

func TestTrayIconEmbedded(t *testing.T) {
	// icon.ico is only linked into the systray-enabled build; this file always
	// compiles. When the enabled file is in the same package build, trayIcon
	// must be a complete ICO (header + image), not the old truncated stub.
	if len(trayIcon) < 100 {
		t.Fatalf("trayIcon too small (%d bytes); need a valid Windows ICO", len(trayIcon))
	}
	if trayIcon[0] != 0 || trayIcon[1] != 0 || trayIcon[2] != 1 || trayIcon[3] != 0 {
		t.Fatalf("trayIcon missing ICO signature, got %v", trayIcon[:4])
	}
}

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
