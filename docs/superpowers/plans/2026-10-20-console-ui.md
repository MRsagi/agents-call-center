# Console UI + Idle Dictation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a localhost browser **Console** (Call / Voicemail / Config) opened from the Status Tray, plus **Idle Dictation** via Feature Code `*2`, backed by config file persistence, status/SSE APIs, and in-process serve restart on config save.

**Architecture:** Extend the existing `acc serve` daemon. Static Console assets are served at `/console`. New REST + SSE endpoints expose config, voicemail mutations, live status, and events. Idle Dictation is a Dialplan action that blocks the runner (so Agent Calls wait) until hang-up, sends a hotkey to the foreground app via `Focuser.Type` (no `Focus`), and surfaces state in the Console. Spec: `docs/superpowers/specs/2026-10-20-console-ui-design.md`.

**Tech Stack:** Go 1.25, `net/http`, `embed` for Console static files, SSE over `text/event-stream`, existing `systray`, OS browser open (`cmd start` / `open`), JSON config under data dir.

## Global Constraints

- Localhost only (`127.0.0.1`), no auth — trust model unchanged
- Domain terms must match `CONTEXT.md` (Console, Idle Dictation, Status Tray, Feature Code, …)
- Console UI platforms: **Windows + macOS**; Linux keeps tray/CLI (omit Open Console or show unsupported)
- Idle Dictation: Feature Code only (default `*2`); no bare pickup; no focus steal; ends on hang-up only; does **not** yield to ringing Agent Calls (handler holds runner lock until hang-up)
- Config: file `{DataDir}/config.json`, then `ACC_*` env overrides (env wins); Console marks env-overridden fields
- Config Save → write file → in-process serve restart; failed re-bind → exit non-zero
- Voicemail Console: text + delete/clear; audio remains phone `*1`
- TDD: failing test → implement → pass → commit per task
- Module path: `github.com/MRsagi/agents-call-center`

## File structure

```
docs/superpowers/specs/2026-10-20-console-ui-design.md
docs/superpowers/plans/2026-10-20-console-ui.md
CONTEXT.md
internal/config/config.go          # + IdleDictationCode, DictationHotkey, file load/save, OverrideSet
internal/config/file.go            # NEW: LoadFile / SaveFile / Merge
internal/config/file_test.go       # NEW
internal/domain/dialplan.go        # + IdleDictation
internal/domain/dialplan_test.go
internal/domain/voicemail.go       # + List / DeleteAll
internal/domain/voicemail_test.go
internal/domain/recent.go          # NEW: RecentCalls ring buffer
internal/domain/recent_test.go     # NEW
internal/domain/events.go          # NEW: Event / Bus
internal/domain/events_test.go     # NEW
internal/focus/recipe.go           # + KeysForChord
internal/focus/recipe_test.go
internal/focus/os_windows.go       # Ctrl/Alt/Shift chord SendKeys
internal/focus/os_darwin.go        # chord keystrokes
internal/line/line.go              # + WaitHangup optional via HangupWaiter
internal/line/fake.go              # WaitHangup + SimulateHangup / idle digits helpers
internal/line/virtual.go           # WaitHangup
internal/line/sip/line.go          # WaitHangup on BYE/on-hook while idle
internal/runner/runner.go          # Idle Dictation session + Deps extras
internal/runner/runner_test.go
internal/app/app.go                # wire recent, events, dictation state, status
internal/api/server.go             # console, config, status, events, voicemail DELETE
internal/api/server_test.go
internal/api/console/              # NEW: embed.FS static UI
  embed.go
  index.html
  app.js
  styles.css
internal/tray/systray_enabled.go   # Open Console menu
internal/tray/open.go              # NEW: OpenConsoleURL
internal/cli/serve.go              # restart loop + OpenConsole wiring
internal/cli/tray_serve.go         # OpenConsole callback
README.md
```

---

### Task 1: Dialplan Idle Dictation

**Files:**
- Modify: `internal/domain/dialplan.go`
- Modify: `internal/domain/dialplan_test.go`
- Modify: `internal/config/config.go` (add `IdleDictationCode`, `DictationHotkey` defaults)

**Interfaces:**
- Produces: `domain.IdleDictation IdleAction`; `Dialplan{VoicemailCode, IdleDictationCode}`; `NewDialplan(vm, dictation string)`; `MatchIdle` returns `IdleDictation` for the dictation code
- Consumes: none

- [ ] **Step 1: Write the failing test**

```go
func TestDialplan_IdleDictation(t *testing.T) {
	d := domain.NewDialplan("*1", "*2")
	if d.MatchIdle("*2") != domain.IdleDictation {
		t.Fatal("want IdleDictation")
	}
	if d.MatchIdle("*1") != domain.IdleVoicemail {
		t.Fatal("want IdleVoicemail")
	}
	if d.MatchIdle("*3") != domain.IdleError {
		t.Fatal("want IdleError")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain -run TestDialplan_IdleDictation -v`  
Expected: FAIL (unknown identifier / wrong `NewDialplan` arity)

- [ ] **Step 3: Implement dialplan + config defaults**

```go
// dialplan.go
const (
	IdleNone IdleAction = iota
	IdleVoicemail
	IdleDictation
	IdleError
)

type Dialplan struct {
	VoicemailCode      string
	IdleDictationCode  string
}

func NewDialplan(voicemailCode, idleDictationCode string) Dialplan {
	if voicemailCode == "" {
		voicemailCode = "*1"
	}
	if idleDictationCode == "" {
		idleDictationCode = "*2"
	}
	return Dialplan{VoicemailCode: voicemailCode, IdleDictationCode: idleDictationCode}
}

func (d Dialplan) MatchIdle(digits string) IdleAction {
	if digits == "" {
		return IdleNone
	}
	if digits == d.VoicemailCode {
		return IdleVoicemail
	}
	if digits == d.IdleDictationCode {
		return IdleDictation
	}
	return IdleError
}
```

Update `config.Default()`:

```go
FeatureCode:        "*1",
IdleDictationCode:  "*2",
DictationHotkey:    "ctrl+m",
```

Add fields on `Config`. Update `app.NewWithAdapters` to call `domain.NewDialplan(cfg.FeatureCode, cfg.IdleDictationCode)`. Fix any compile breaks from old `NewDialplan` single-arg call sites.

Update existing `TestDialplan_VoicemailCode` to `NewDialplan("*1", "*2")`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/domain ./internal/app ./internal/runner ./internal/api -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/dialplan.go internal/domain/dialplan_test.go internal/config/config.go internal/app/app.go
git commit -m "feat: add Idle Dictation Feature Code to Dialplan"
```

---

### Task 2: Hotkey chord → Focuser.Type keys

**Files:**
- Modify: `internal/focus/recipe.go`
- Modify: `internal/focus/recipe_test.go`
- Modify: `internal/focus/os_windows.go`
- Modify: `internal/focus/os_darwin.go` (chord support)

**Interfaces:**
- Produces: `focus.KeysForChord(chord string) ([]string, error)` — e.g. `"ctrl+m"` → `[]string{"Ctrl+m"}` (single SendKeys token)
- Consumes: existing `Focuser.Type`

- [ ] **Step 1: Write the failing test**

```go
func TestKeysForChord_CtrlM(t *testing.T) {
	keys, err := KeysForChord("ctrl+m")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "Ctrl+m" {
		t.Fatalf("got %#v", keys)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/focus -run TestKeysForChord_CtrlM -v`  
Expected: FAIL

- [ ] **Step 3: Implement KeysForChord + OS mapping**

```go
func KeysForChord(chord string) ([]string, error) {
	c := strings.ToLower(strings.TrimSpace(chord))
	if c == "" {
		return nil, errors.New("empty chord")
	}
	parts := strings.Split(c, "+")
	if len(parts) == 0 {
		return nil, errors.New("invalid chord")
	}
	key := parts[len(parts)-1]
	mods := parts[:len(parts)-1]
	var b strings.Builder
	for _, m := range mods {
		switch m {
		case "ctrl", "control":
			b.WriteString("Ctrl+")
		case "alt":
			b.WriteString("Alt+")
		case "shift":
			b.WriteString("Shift+")
		case "cmd", "meta", "super":
			b.WriteString("Cmd+")
		default:
			return nil, fmt.Errorf("unknown modifier %q", m)
		}
	}
	b.WriteString(key)
	return []string{b.String()}, nil
}
```

In `windowsSendKey`, handle tokens containing `Ctrl+` / `Alt+` / `Shift+` by emitting SendKeys modifiers (`^`, `%`, `+`) **without** escaping those chars, then the final key. Example: `"Ctrl+m"` → `"^m"`.

On Darwin, map `Ctrl+m` / `Cmd+…` via existing osascript keystroke path (extend whatever `Type` already uses).

Idle Dictation must call `Type` **without** `Focus` so keystrokes hit the foreground window.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/focus -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/focus/
git commit -m "feat: parse dictation hotkey chords for Focuser.Type"
```

---

### Task 3: Voicemail List + DeleteAll

**Files:**
- Modify: `internal/domain/voicemail.go`
- Modify: `internal/domain/voicemail_test.go`

**Interfaces:**
- Produces: `VoicemailStore.List() ([]VoicemailEntry, error)`; `DeleteAll() error` (in addition to existing `MarkHeard`)
- Consumes: existing JSONL store

- [ ] **Step 1: Write the failing test**

```go
func TestVoicemail_ListAndDeleteAll(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenVoicemailStore(filepath.Join(dir, "vm.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	c1, _ := NewCall("one", ModeDTMF, FocusTarget{App: "A"}, 1)
	c2, _ := NewCall("two", ModeDTMF, FocusTarget{App: "B"}, 1)
	_ = s.Leave(c1)
	_ = s.Leave(c2)
	list, err := s.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if err := s.DeleteAll(); err != nil {
		t.Fatal(err)
	}
	n, _ := s.Count()
	if n != 0 {
		t.Fatalf("count=%d", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain -run TestVoicemail_ListAndDeleteAll -v`  
Expected: FAIL

- [ ] **Step 3: Implement List + DeleteAll on interface and jsonl store**

```go
type VoicemailStore interface {
	Leave(*Call) error
	OldestUnheard() (*VoicemailEntry, error)
	MarkHeard(id string) error
	List() ([]VoicemailEntry, error)
	DeleteAll() error
	Count() (int, error)
}

func (s *jsonlVoicemailStore) List() ([]VoicemailEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAll()
}

func (s *jsonlVoicemailStore) DeleteAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
```

Keep `MarkHeard` as delete-one (already).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/domain -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/voicemail.go internal/domain/voicemail_test.go
git commit -m "feat: list and clear all Voicemail entries"
```

---

### Task 4: RecentCalls + Event bus

**Files:**
- Create: `internal/domain/recent.go`
- Create: `internal/domain/recent_test.go`
- Create: `internal/domain/events.go`
- Create: `internal/domain/events_test.go`

**Interfaces:**
- Produces: `RecentCalls` with `Add(*Call)`, `Snapshot() []*Call` (cap 20, newest first)
- Produces: `Event{Type string, Payload any, At time.Time}`; `Bus` with `Publish(Event)`, `Subscribe(ctx) <-chan Event`
- Consumes: `domain.Call`

- [ ] **Step 1: Write failing tests**

```go
func TestRecentCalls_Cap(t *testing.T) {
	r := NewRecentCalls(20)
	for i := 0; i < 25; i++ {
		c, _ := NewCall(fmt.Sprintf("p%d", i), ModeDTMF, FocusTarget{App: "X"}, 1)
		c.Status = StatusCompleted
		r.Add(c)
	}
	if got := len(r.Snapshot()); got != 20 {
		t.Fatalf("len=%d", got)
	}
}

func TestBus_PublishSubscribe(t *testing.T) {
	b := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx)
	b.Publish(Event{Type: "call.updated"})
	select {
	case ev := <-ch:
		if ev.Type != "call.updated" {
			t.Fatalf("%v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/domain -run 'TestRecentCalls_Cap|TestBus_PublishSubscribe' -v`  
Expected: FAIL

- [ ] **Step 3: Implement RecentCalls and Bus**

Ring buffer: mutex + slice, prepend on Add, truncate to cap.  
Bus: mutex + map of subscriber chans (buffer 16); drop on full; unsubscribe on ctx cancel; non-blocking publish.

Event type constants (strings): `call.updated`, `voicemail.changed`, `dictation.started`, `dictation.stopped`, `restart.soon`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/domain -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/recent.go internal/domain/recent_test.go internal/domain/events.go internal/domain/events_test.go
git commit -m "feat: add RecentCalls buffer and SSE event bus"
```

---

### Task 5: Config file load/save + env override markers

**Files:**
- Create: `internal/config/file.go`
- Create: `internal/config/file_test.go`
- Modify: `internal/config/config.go` (`ApplyEnv` records overridden keys)

**Interfaces:**
- Produces: `LoadFile(path string) (Config, error)`; `SaveFile(path string, cfg Config) error`; `LoadEffective(dataDir string) (cfg Config, envOverrides map[string]bool, err error)` — Default → file merge → ApplyEnv; `envOverrides` keys are JSON field names used by Console
- Consumes: `Config`, `ApplyEnv`

- [ ] **Step 1: Write failing test**

```go
func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.FeatureCode = "*9"
	cfg.IdleDictationCode = "*8"
	cfg.DictationHotkey = "ctrl+m"
	if err := SaveFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.FeatureCode != "*9" || got.IdleDictationCode != "*8" {
		t.Fatalf("%+v", got)
	}
}

func TestEnvOverridesWin(t *testing.T) {
	t.Setenv("ACC_LINE_BACKEND", "sip")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.LineBackend = LineBackendFake
	_ = SaveFile(path, cfg)
	effective, overrides, err := LoadEffective(dir)
	if err != nil {
		t.Fatal(err)
	}
	if effective.LineBackend != LineBackendSIP {
		t.Fatalf("backend=%s", effective.LineBackend)
	}
	if !overrides["line_backend"] {
		t.Fatal("expected line_backend override mark")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config -v`  
Expected: FAIL

- [ ] **Step 3: Implement JSON file + LoadEffective**

Serialize all `Config` fields with snake_case JSON tags (add tags to `Config` or use a DTO). Durations as string (e.g. `"90s"`) or milliseconds — pick milliseconds int for simplicity in JSON:

```go
type fileDTO struct {
	HTTPBind           string `json:"http_bind"`
	DataDir            string `json:"data_dir"`
	RingTimeoutMS      int64  `json:"ring_timeout_ms"`
	LineRecoverWaitMS  int64  `json:"line_recover_wait_ms"`
	FeatureCode        string `json:"feature_code"`
	IdleDictationCode  string `json:"idle_dictation_code"`
	DictationHotkey    string `json:"dictation_hotkey"`
	LineBackend        string `json:"line_backend"`
	SynthBackend       string `json:"synth_backend"`
	FocusBackend       string `json:"focus_backend"`
	VirtualControlBind string `json:"virtual_control_bind"`
	SIPBind            string `json:"sip_bind"`
	SIPUser            string `json:"sip_user"`
	SIPPassword        string `json:"sip_password"`
	SIPRealm           string `json:"sip_realm"`
	SIPExtension       string `json:"sip_extension"`
	SIPPublicHost      string `json:"sip_public_host"`
	PhoneMicEnabled    bool   `json:"phone_mic_enabled"`
	PhoneMicDevice     string `json:"phone_mic_device"`
	TrayEnabled        *bool  `json:"tray_enabled"` // nil = default on
}
```

`ApplyEnv` should return or fill `map[string]bool` of overridden JSON keys. `LoadEffective(dataDir)`: start Default, if `{dataDir}/config.json` exists merge, then ApplyEnv.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: persist Call Center config JSON with env override marks"
```

---

### Task 6: Line WaitHangup + Idle Dictation in Runner

**Files:**
- Modify: `internal/line/line.go`
- Modify: `internal/line/fake.go` (+ test helpers)
- Modify: `internal/line/virtual.go`
- Modify: `internal/line/sip/line.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`
- Modify: `internal/app/app.go` (wire Deps: Events, Recent, DictationHotkey, PhoneMic warn flag, IdleDictating atomic)

**Interfaces:**
- Produces: `line.HangupWaiter` interface `{ WaitHangup(ctx context.Context) error }`; Fake: `TriggerIdleDigits(digits)`, `TriggerHangup()`
- Produces: runner handles `IdleDictation`: set dictating → Type hotkey (no Focus) → `WaitHangup` → Type hotkey again → clear dictating; publish events; **blocks** until hang-up (holds processing lock)
- Consumes: `KeysForChord`, Dialplan `IdleDictation`, Event bus, PhoneMic (already started at app level)

- [ ] **Step 1: Write failing runner test**

```go
func TestRunner_IdleDictationBlocksUntilHangup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ln := line.NewFake(line.FakeScript{})
	foc := &focus.Fake{}
	// wire runner with dialplan *2, hotkey ctrl+m, fake line/focus/vm
	// start runner
	// TriggerIdleDigits("*2")
	// assert foc.Typed has Ctrl+m within 500ms
	// enqueue a call — must NOT start ringing while dictating
	// TriggerHangup()
	// assert second Type and call then processes
}
```

(Fill exact wiring matching existing `runner_test.go` patterns.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner -run TestRunner_IdleDictationBlocksUntilHangup -v`  
Expected: FAIL

- [ ] **Step 3: Implement WaitHangup + runner case**

```go
// line.go
type HangupWaiter interface {
	WaitHangup(ctx context.Context) error
}
```

Fake: channel closed on `TriggerHangup`. SIP: wait for dialog BYE / on-hook while not in an Agent Call. Virtual: wait on existing hangup signal.

```go
case domain.IdleDictation:
	r.setDictating(true)
	r.publish(domain.Event{Type: "dictation.started"})
	keys, err := focus.KeysForChord(r.deps.DictationHotkey)
	if err == nil {
		_ = r.deps.Focus.Type(ctx, keys) // NO Focus()
	}
	if hw, ok := r.deps.Line.(line.HangupWaiter); ok {
		_ = hw.WaitHangup(ctx)
	}
	if keys != nil {
		_ = r.deps.Focus.Type(ctx, keys)
	}
	r.setDictating(false)
	r.publish(domain.Event{Type: "dictation.stopped"})
```

On Call completion paths, `Recent.Add(c)` + `Publish call.updated`. Expose `App.IdleDictating() bool` for status API.

If Line lacks `HangupWaiter`, log error and return immediately (tests must use Fake with waiter).

- [ ] **Step 4: Run tests**

Run: `go test ./internal/runner ./internal/line/... -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/line/ internal/runner/ internal/app/app.go
git commit -m "feat: Idle Dictation session waits for hang-up"
```

---

### Task 7: HTTP API — status, config, voicemail DELETE, SSE, console stub

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/server_test.go`
- Create: `internal/api/console/embed.go` (minimal `index.html` placeholder OK; full UI in Task 8)

**Interfaces:**
- Produces routes:
  - `GET /v1/status` → `{line, queue_depth, active_call, idle_dictating, recent, voicemail_count, dictation_warning?}`
  - `GET /v1/config` → `{config, env_overrides}`
  - `PUT /v1/config` → validate, `SaveFile`, signal restart (callback on Server)
  - `GET /v1/voicemail` → `{count, entries: [...]}` (expand beyond oldest-only)
  - `DELETE /v1/voicemail/{id}` → `MarkHeard`
  - `DELETE /v1/voicemail` → `DeleteAll`
  - `GET /v1/events` → SSE
  - `GET /console/` → embedded index
- Consumes: App methods for status/config/restart

- [ ] **Step 1: Write failing API tests**

Cover: GET status shape; PUT config writes file and invokes restart hook; DELETE voicemail id; DELETE all; GET voicemail returns entries list; SSE receives an event after Publish.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api -v`  
Expected: FAIL on new cases

- [ ] **Step 3: Implement handlers**

```go
type Server struct {
	app     *app.App
	mux     *http.ServeMux
	OnRestart func() // set by serve loop
}

// PUT /v1/config
// decode Config DTO → SaveFile(filepath.Join(dataDir,"config.json")) → if OnRestart != nil { OnRestart() }
```

SSE: set headers `Content-Type: text/event-stream`, `Cache-Control: no-cache`, subscribe to bus, write `event: …\ndata: …\n\n`, flush.

Expand GET voicemail to return full `entries` from `List()`.

`GET /console/` and `/console/*` via `http.FileServer` on `embed.FS`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/api -count=1`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat: Console APIs for status, config, voicemail, SSE"
```

---

### Task 8: Console UI (Call | Voicemail | Config)

**Files:**
- Create/replace: `internal/api/console/index.html`
- Create: `internal/api/console/app.js`
- Create: `internal/api/console/styles.css`
- Modify: `internal/api/console/embed.go`

**Interfaces:**
- Produces: single-page UI with three nav views; polls `/v1/status` every 1s and upgrades to `/v1/events` when available; config form GET/PUT; voicemail list DELETE
- Consumes: Task 7 APIs

- [ ] **Step 1: Build static UI**

Vanilla HTML/CSS/JS (no npm). Structure:

- Nav: Call | Voicemail | Config
- Call: large `#prompt`, status chip, meta, `#dictation-banner` hidden unless `idle_dictating`, `#recent` list
- Voicemail: `#vm-list` with delete buttons + Clear all
- Config: form fields for full config DTO; show badge on env-overridden fields; Save button → PUT → show “Reconnecting…” and poll `/v1/health` until OK then reload config

Keep styling simple, readable, dark-or-light neutral desk tool (not marketing). No external CDN deps.

- [ ] **Step 2: Manual sanity (optional in CI)**

Run: `go test ./internal/api -count=1` still PASS (embed compiles).

- [ ] **Step 3: Commit**

```bash
git add internal/api/console/
git commit -m "feat: add browser Console pages for Call, Voicemail, Config"
```

---

### Task 9: Tray Open Console + serve restart loop

**Files:**
- Create: `internal/tray/open.go`
- Modify: `internal/tray/systray_enabled.go`
- Modify: `internal/cli/tray_serve.go`
- Modify: `internal/cli/serve.go`
- Modify: `README.md`

**Interfaces:**
- Produces: tray menu **Open Console**; `tray.OpenURL(url string) error` (`rundll32 url.dll,FileProtocolHandler` on Windows; `open` on macOS; no-op/log on Linux)
- Produces: `runServe` loop: load config → start app+HTTP → wait cancel **or** restart signal → shutdown → if restart continue else return
- Consumes: `api.Server.OnRestart`, HTTP bind from config

- [ ] **Step 1: Write tray open unit test (URL formatting) if pure**

```go
func TestConsoleURL(t *testing.T) {
	if got := ConsoleURL("127.0.0.1:8741"); got != "http://127.0.0.1:8741/console/" {
		t.Fatal(got)
	}
}
```

- [ ] **Step 2: Implement Open Console menu**

On click: `OpenURL(ConsoleURL(cfg.HTTPBind))`. On Linux (`runtime.GOOS == "linux"`): do not add the menu item.

Wire `serveTrayProvider` with HTTP bind string from current config.

- [ ] **Step 3: Implement serve restart loop**

```go
for {
	cfg, _, err := config.LoadEffective(config.Default().DataDir) // or honor ACC_DATA_DIR if present
	// force OS synth/focus as today
	a, err := app.New(cfg)
	// ...
	apiSrv := api.NewServer(a)
	restart := make(chan struct{}, 1)
	apiSrv.OnRestart = func() {
		select { case restart <- struct{}{}: default: }
		// publish restart.soon via a.Events
	}
	// ListenAndServe in goroutine
	select {
	case <-runCtx.Done():
		shutdown; return nil
	case <-restart:
		shutdown; continue
	case err := <-errCh:
		return err
	}
}
```

On failed `ListenAndServe` bind after restart: return error (process exits).

- [ ] **Step 4: Run full test suite**

Run: `go test ./... -count=1`  
Expected: PASS

- [ ] **Step 5: Update README**

Document: tray Open Console; config file path; Idle Dictation `*2` + hotkey; Console pages; env still overrides.

- [ ] **Step 6: Commit**

```bash
git add internal/tray/ internal/cli/serve.go internal/cli/tray_serve.go README.md
git commit -m "feat: Open Console from tray and restart serve on config save"
```

---

### Task 10: Manual verification checklist (no code)

**Files:** none (operator checklist)

- [ ] **Step 1: Windows desk**

1. `acc serve` with SIP + Phone Mic as today  
2. Tray → Open Console → browser shows Call page  
3. `acc call --prompt "test UI" --app Notepad --digits 1` → Console shows prompt/state  
4. Miss a call → Voicemail page lists prompt; Delete works; phone `*1` still plays if any remain  
5. Config: change Idle Dictation code or hotkey → Save → reconnect; PAP2T may need re-REGISTER if SIP fields changed  
6. Focus Cursor, go off-hook, dial `*2`, speak → dictation; hang up → stops; queued Agent Call runs after  

- [ ] **Step 2: macOS smoke** (if available): Open Console + `*2` hotkey path  

- [ ] **Step 3: Commit docs only if checklist notes needed** — otherwise done  

---

## Spec coverage (self-review)

| Spec requirement | Task |
|---|---|
| Hybrid tray + Open Console | 9 |
| Browser Console at `/console` | 7–8 |
| Call page: prompt, state, recent, dictation banner | 4, 7, 8 |
| Voicemail list + delete/clear | 3, 7, 8 |
| Full config form + secrets | 5, 7, 8 |
| Save → file → in-process restart | 5, 7, 9 |
| Env overrides marked | 5, 8 |
| Idle Dictation `*2`, no pickup auto | 1, 6 |
| Hotkey to foreground, no Focus | 2, 6 |
| End on hang-up only; Agents wait | 6 |
| SSE + poll fallback | 7, 8 |
| Win+mac Console; Linux omit Open Console | 8, 9 |
| Localhost, no auth | global |
| Failed re-bind exits | 9 |

## Placeholder scan

No TBD/TODO left in task steps; restart and WaitHangup behaviors are specified.

## Type consistency

- `NewDialplan(vm, dictation string)` used from Task 1 onward  
- Config fields `IdleDictationCode`, `DictationHotkey` from Task 1/5  
- Event type strings shared by Tasks 4, 6, 7  
- `HangupWaiter.WaitHangup` used by Task 6  
- `OnRestart` callback on `api.Server` used by Tasks 7 and 9  
