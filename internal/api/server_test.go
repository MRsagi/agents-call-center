package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/api"
	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/domain"
	"github.com/MRsagi/agents-call-center/internal/focus"
	"github.com/MRsagi/agents-call-center/internal/line"
	"github.com/MRsagi/agents-call-center/internal/synth"
)

func TestAPI_EnqueueAndPollUntilCompleted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.RingTimeout = time.Second

	ln := line.NewFake(line.FakeScript{AnswerAfter: time.Millisecond, Digits: "3"})
	a, err := app.NewWithAdapters(cfg, app.Adapters{Line: ln, Synth: &synth.Fake{}, Focus: &focus.Fake{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Stop()

	srv := httptest.NewServer(api.NewServer(a).Handler())
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{
		"prompt":       "approve?",
		"mode":         "dtmf",
		"max_digits":   1,
		"focus_target": map[string]string{"app": "Cursor"},
	})
	resp, err := http.Post(srv.URL+"/v1/calls", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status=%d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("empty id")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		getResp, err := http.Get(srv.URL + "/v1/calls/" + created.ID)
		if err != nil {
			t.Fatal(err)
		}
		var c domain.Call
		_ = json.NewDecoder(getResp.Body).Decode(&c)
		getResp.Body.Close()
		if c.Status == domain.StatusCompleted {
			if c.Digits != "3" {
				t.Fatalf("digits=%q", c.Digits)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for completed")
}

func TestAPI_HealthAndVoicemailEmpty(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	a, err := app.NewWithAdapters(cfg, app.Adapters{
		Line: line.NewFake(line.FakeScript{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.NewServer(a).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var health struct {
		OK   bool   `json:"ok"`
		Line string `json:"line"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if !health.OK || health.Line != "up" {
		t.Fatalf("health=%+v", health)
	}

	vmResp, err := http.Get(srv.URL + "/v1/voicemail")
	if err != nil {
		t.Fatal(err)
	}
	defer vmResp.Body.Close()
	var vm struct {
		Count  int  `json:"count"`
		Oldest *any `json:"oldest"`
	}
	if err := json.NewDecoder(vmResp.Body).Decode(&vm); err != nil {
		t.Fatal(err)
	}
	if vm.Count != 0 || vm.Oldest != nil {
		t.Fatalf("voicemail=%+v", vm)
	}
}
