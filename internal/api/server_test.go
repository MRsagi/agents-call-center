package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
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
		Count   int                      `json:"count"`
		Oldest  *any                     `json:"oldest"`
		Entries []domain.VoicemailEntry  `json:"entries"`
	}
	if err := json.NewDecoder(vmResp.Body).Decode(&vm); err != nil {
		t.Fatal(err)
	}
	if vm.Count != 0 || vm.Oldest != nil || len(vm.Entries) != 0 {
		t.Fatalf("voicemail=%+v", vm)
	}
}

func testApp(t *testing.T) *app.App {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	a, err := app.NewWithAdapters(cfg, app.Adapters{
		Line:  line.NewFake(line.FakeScript{}),
		Synth: &synth.Fake{},
		Focus: &focus.Fake{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAPI_StatusShape(t *testing.T) {
	a := testApp(t)
	srv := httptest.NewServer(api.NewServer(a).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var st struct {
		Line             string          `json:"line"`
		QueueDepth       int             `json:"queue_depth"`
		ActiveCall       *domain.Call    `json:"active_call"`
		IdleDictating    bool            `json:"idle_dictating"`
		Recent           []*domain.Call  `json:"recent"`
		VoicemailCount   int             `json:"voicemail_count"`
		DictationWarning *bool           `json:"dictation_warning"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Line != "up" {
		t.Fatalf("line=%q", st.Line)
	}
	if st.QueueDepth != 0 {
		t.Fatalf("queue_depth=%d", st.QueueDepth)
	}
	if st.ActiveCall != nil {
		t.Fatalf("active_call=%+v", st.ActiveCall)
	}
	if st.IdleDictating {
		t.Fatal("expected idle_dictating false")
	}
	if st.Recent == nil {
		t.Fatal("recent should be present (empty list ok)")
	}
	if st.VoicemailCount != 0 {
		t.Fatalf("voicemail_count=%d", st.VoicemailCount)
	}
}

func TestAPI_PutConfigWritesAndRestarts(t *testing.T) {
	a := testApp(t)
	apiSrv := api.NewServer(a)
	var restarted atomic.Bool
	apiSrv.OnRestart = func() { restarted.Store(true) }
	srv := httptest.NewServer(apiSrv.Handler())
	defer srv.Close()

	body := []byte(`{"feature_code":"*9","idle_dictation_code":"*8","dictation_hotkey":"alt+d"}`)
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/v1/config", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT status=%d body=%s", resp.StatusCode, b)
	}
	if !restarted.Load() {
		t.Fatal("expected OnRestart to be called")
	}
	path := filepath.Join(a.Config.DataDir, "config.json")
	loaded, err := config.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FeatureCode != "*9" || loaded.IdleDictationCode != "*8" || loaded.DictationHotkey != "alt+d" {
		t.Fatalf("loaded=%+v", loaded)
	}

	getResp, err := http.Get(srv.URL + "/v1/config")
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	var getBody struct {
		Config       map[string]any   `json:"config"`
		EnvOverrides map[string]bool  `json:"env_overrides"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatal(err)
	}
	if getBody.Config == nil {
		t.Fatal("missing config")
	}
	if getBody.EnvOverrides == nil {
		t.Fatal("missing env_overrides")
	}
	if getBody.Config["feature_code"] != "*9" {
		t.Fatalf("config.feature_code=%v", getBody.Config["feature_code"])
	}
}

func TestAPI_VoicemailEntriesAndDelete(t *testing.T) {
	a := testApp(t)
	c1, err := domain.NewCall("first", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := domain.NewCall("second", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Voicemail.Leave(c1); err != nil {
		t.Fatal(err)
	}
	if err := a.Voicemail.Leave(c2); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(api.NewServer(a).Handler())
	defer srv.Close()

	listResp, err := http.Get(srv.URL + "/v1/voicemail")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	var list struct {
		Count   int                     `json:"count"`
		Entries []domain.VoicemailEntry `json:"entries"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Count != 2 || len(list.Entries) != 2 {
		t.Fatalf("list=%+v", list)
	}

	delReq, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/voicemail/"+c1.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent && delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE id status=%d", delResp.StatusCode)
	}

	listResp2, err := http.Get(srv.URL + "/v1/voicemail")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp2.Body.Close()
	var list2 struct {
		Count   int                     `json:"count"`
		Entries []domain.VoicemailEntry `json:"entries"`
	}
	_ = json.NewDecoder(listResp2.Body).Decode(&list2)
	if list2.Count != 1 || len(list2.Entries) != 1 || list2.Entries[0].CallID != c2.ID {
		t.Fatalf("after delete one: %+v", list2)
	}

	delAll, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/voicemail", nil)
	if err != nil {
		t.Fatal(err)
	}
	delAllResp, err := http.DefaultClient.Do(delAll)
	if err != nil {
		t.Fatal(err)
	}
	delAllResp.Body.Close()
	if delAllResp.StatusCode != http.StatusNoContent && delAllResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE all status=%d", delAllResp.StatusCode)
	}
	n, err := a.Voicemail.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("count after DeleteAll=%d", n)
	}
}

func TestAPI_SSEReceivesEvent(t *testing.T) {
	a := testApp(t)
	srv := httptest.NewServer(api.NewServer(a).Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE status=%d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type=%q", ct)
	}

	got := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		var eventType string
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			}
			if strings.HasPrefix(line, "data:") && eventType != "" {
				got <- eventType
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	a.Events.Publish(domain.Event{Type: domain.EventCallUpdated, Payload: map[string]string{"id": "x"}})

	select {
	case typ := <-got:
		if typ != domain.EventCallUpdated {
			t.Fatalf("event type=%q", typ)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for SSE event")
	}
}

func TestAPI_ConsoleStub(t *testing.T) {
	a := testApp(t)
	srv := httptest.NewServer(api.NewServer(a).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/console/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("console status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("Console")) && !bytes.Contains(bytes.ToLower(body), []byte("html")) {
		t.Fatalf("unexpected console body: %s", body)
	}
}
