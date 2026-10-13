# Agents Call Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a Go desk daemon that Agents enqueue Calls into (HTTP/CLI/MCP); it rings a Line (Fake → Virtual → SIP/PAP2T), speaks Prompts via OS TTS, collects DTMF, focuses a Focus Target, types digits+Enter, and stores unheard Voicemail in JSONL — plus macOS Status Tray and Phone Mic.

**Architecture:** Single `acc serve` process: Surfaces → FIFO Queue → Call runner → adapters (`Line`, `Synthesizer`, `Focuser`, `PhoneMic`). Fake Line unlocks CI; Virtual Line unlocks manual audio; SIP registers PAP2T. See spec `docs/superpowers/specs/2026-10-13-agents-call-center-design.md` and `CONTEXT.md`.

**Tech Stack:** Go 1.22+, `net/http`, `github.com/spf13/cobra` (CLI), MCP SDK for Go (`github.com/modelcontextprotocol/go-sdk` or current official), `github.com/getlantern/systray`, `github.com/emiago/sipgo` (SIP), OS TTS via `exec` (`say` / PowerShell SAPI / `espeak-ng`), BlackHole (macOS) as Phone Mic sink for v1.

## Global Constraints

- Localhost only (`127.0.0.1`), no auth
- Agents enqueue and return immediately — never block for Reply on the HTTP/MCP call
- Call Mode API includes `notify` | `dtmf` | `speech`; implement **`dtmf` only** in v1 (reject or no-op other modes with clear error)
- Queue is strict FIFO; one active Call
- DTMF Reply delivery: type digits then Enter into Focus Target
- Voicemail: ring timeout default 90s; Feature Code default `*1`; JSONL until heard
- Idle pickup = dial tone only (no auto-focus, no auto-voicemail) in v1
- Phone Mic: macOS only in v1
- Domain terms must match `CONTEXT.md`
- TDD: failing test → implement → pass → commit per task
- Module path: `github.com/MRsagi/agents-call-center`

## File structure

```
CONTEXT.md
docs/superpowers/specs/2026-10-13-agents-call-center-design.md
docs/superpowers/plans/2026-10-13-agents-call-center.md
go.mod
go.sum
cmd/acc/main.go
internal/config/config.go
internal/domain/call.go
internal/domain/queue.go
internal/domain/queue_test.go
internal/domain/voicemail.go
internal/domain/voicemail_test.go
internal/domain/dialplan.go
internal/domain/dialplan_test.go
internal/focus/focus.go
internal/focus/recipe.go
internal/focus/recipe_test.go
internal/focus/fake.go
internal/focus/os_darwin.go
internal/focus/os_other.go
internal/synth/synth.go
internal/synth/fake.go
internal/synth/os.go
internal/line/line.go
internal/line/fake.go
internal/line/fake_test.go
internal/line/virtual.go
internal/line/sip/line.go
internal/runner/runner.go
internal/runner/runner_test.go
internal/api/server.go
internal/api/server_test.go
internal/mcp/server.go
internal/tray/tray.go
internal/phonemic/phonemic.go
internal/phonemic/macos.go
internal/phonemic/stub.go
internal/app/app.go
README.md
```

---

### Task 1: Module scaffold + domain Call types

**Files:**
- Create: `go.mod`
- Create: `internal/domain/call.go`
- Create: `internal/domain/call_test.go`
- Create: `cmd/acc/main.go`

**Interfaces:**
- Produces: `domain.Call`, `domain.Status`, `domain.Mode`, `domain.FocusTarget`, `domain.NewCall(...)`, `domain.DeliveryFocusMissed`

- [ ] **Step 1: Initialize module**

```bash
go mod init github.com/MRsagi/agents-call-center
```

Expected: `go.mod` exists with that module path.

- [ ] **Step 2: Write failing tests for Call defaults**

Create `internal/domain/call_test.go`:

```go
package domain_test

import (
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestNewCall_Defaults(t *testing.T) {
	ft := domain.FocusTarget{App: "Cursor", TitleContains: "agent"}
	c, err := domain.NewCall("approve deploy", domain.ModeDTMF, ft, 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if c.Mode != domain.ModeDTMF {
		t.Fatalf("mode=%s", c.Mode)
	}
	if c.MaxDigits != 1 {
		t.Fatalf("max_digits=%d want 1", c.MaxDigits)
	}
	if c.Status != domain.StatusQueued {
		t.Fatalf("status=%s", c.Status)
	}
	if c.CreatedAt.After(time.Now().Add(time.Second)) {
		t.Fatal("CreatedAt in future")
	}
}

func TestNewCall_RejectsEmptyPrompt(t *testing.T) {
	_, err := domain.NewCall("", domain.ModeDTMF, domain.FocusTarget{}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewCall_RejectsSpeechModeInV1(t *testing.T) {
	_, err := domain.NewCall("hi", domain.ModeSpeech, domain.FocusTarget{}, 1)
	if err == nil {
		t.Fatal("expected error for unimplemented mode")
	}
}
```

- [ ] **Step 3: Run tests — expect FAIL**

```bash
go test ./internal/domain/ -v
```

Expected: FAIL — package or types not defined.

- [ ] **Step 4: Implement domain types**

Create `internal/domain/call.go`:

```go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusQueued     Status = "queued"
	StatusRinging    Status = "ringing"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusVoicemail  Status = "voicemail"
	StatusFailed     Status = "failed"
)

type Mode string

const (
	ModeNotify Mode = "notify"
	ModeDTMF   Mode = "dtmf"
	ModeSpeech Mode = "speech"
)

type Delivery string

const (
	DeliveryOK          Delivery = "ok"
	DeliveryFocusMissed Delivery = "focus_missed"
)

type FocusTarget struct {
	App           string `json:"app"`
	TitleContains string `json:"title_contains,omitempty"`
	ChatID        string `json:"chat_id,omitempty"`
}

type Call struct {
	ID          string      `json:"id"`
	Prompt      string      `json:"prompt"`
	Mode        Mode        `json:"mode"`
	MaxDigits   int         `json:"max_digits"`
	FocusTarget FocusTarget `json:"focus_target"`
	CreatedAt   time.Time   `json:"created_at"`
	Status      Status      `json:"status"`
	Digits      string      `json:"digits,omitempty"`
	Delivery    Delivery    `json:"delivery,omitempty"`
	Error       string      `json:"error,omitempty"`
}

func NewCall(prompt string, mode Mode, ft FocusTarget, maxDigits int) (*Call, error) {
	if prompt == "" {
		return nil, errors.New("prompt required")
	}
	switch mode {
	case ModeDTMF:
		// ok — only implemented mode in v1
	case ModeNotify, ModeSpeech:
		return nil, errors.New("mode not implemented in v1: " + string(mode))
	default:
		return nil, errors.New("unknown mode")
	}
	if maxDigits <= 0 {
		maxDigits = 1
	}
	return &Call{
		ID:          uuid.NewString(),
		Prompt:      prompt,
		Mode:        mode,
		MaxDigits:   maxDigits,
		FocusTarget: ft,
		CreatedAt:   time.Now().UTC(),
		Status:      StatusQueued,
	}, nil
}
```

- [ ] **Step 5: Add uuid dependency and stub main**

```bash
go get github.com/google/uuid
```

Create `cmd/acc/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("acc: use subcommands after Task 6+")
}
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
go test ./internal/domain/ -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum cmd/acc/main.go internal/domain/call.go internal/domain/call_test.go
git commit -m "feat: add Call domain types and module scaffold"
```

---

### Task 2: FIFO Queue

**Files:**
- Create: `internal/domain/queue.go`
- Create: `internal/domain/queue_test.go`

**Interfaces:**
- Consumes: `domain.Call`, `domain.Status*`
- Produces: `domain.Queue` with `Enqueue(*Call) error`, `Dequeue() (*Call, bool)`, `Get(id string) (*Call, bool)`, `Len() int`, `Snapshot() []*Call`

- [ ] **Step 1: Write failing queue tests**

```go
package domain_test

import (
	"testing"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestQueue_FIFO(t *testing.T) {
	q := domain.NewQueue()
	a, _ := domain.NewCall("a", domain.ModeDTMF, domain.FocusTarget{}, 1)
	b, _ := domain.NewCall("b", domain.ModeDTMF, domain.FocusTarget{}, 1)
	if err := q.Enqueue(a); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(b); err != nil {
		t.Fatal(err)
	}
	if q.Len() != 2 {
		t.Fatalf("len=%d", q.Len())
	}
	got, ok := q.Dequeue()
	if !ok || got.ID != a.ID {
		t.Fatalf("first dequeue want %s got %#v", a.ID, got)
	}
	got, ok = q.Dequeue()
	if !ok || got.ID != b.ID {
		t.Fatalf("second dequeue want %s", b.ID)
	}
	_, ok = q.Dequeue()
	if ok {
		t.Fatal("expected empty")
	}
}

func TestQueue_GetByID(t *testing.T) {
	q := domain.NewQueue()
	c, _ := domain.NewCall("x", domain.ModeDTMF, domain.FocusTarget{}, 1)
	_ = q.Enqueue(c)
	got, ok := q.Get(c.ID)
	if !ok || got.Prompt != "x" {
		t.Fatalf("get failed: %#v", got)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/domain/ -run Queue -v
```

Expected: FAIL — `NewQueue` undefined.

- [ ] **Step 3: Implement Queue**

```go
package domain

import "sync"

type Queue struct {
	mu      sync.Mutex
	order   []string
	byID    map[string]*Call
}

func NewQueue() *Queue {
	return &Queue{byID: make(map[string]*Call)}
}

func (q *Queue) Enqueue(c *Call) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.byID[c.ID]; ok {
		return nil
	}
	q.byID[c.ID] = c
	q.order = append(q.order, c.ID)
	return nil
}

func (q *Queue) Dequeue() (*Call, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.order) == 0 {
		return nil, false
	}
	id := q.order[0]
	q.order = q.order[1:]
	c := q.byID[id]
	return c, true
}

func (q *Queue) Get(id string) (*Call, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	c, ok := q.byID[id]
	return c, ok
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.order)
}

func (q *Queue) Snapshot() []*Call {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Call, 0, len(q.order))
	for _, id := range q.order {
		out = append(out, q.byID[id])
	}
	return out
}
```

Note: `byID` retains completed Calls for `Get` after dequeue — keep them in `byID` when dequeuing (do not delete). Only remove from `order`.

- [ ] **Step 4: Run — expect PASS**

```bash
go test ./internal/domain/ -run Queue -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/domain/queue.go internal/domain/queue_test.go
git commit -m "feat: add FIFO Call Queue"
```

---

### Task 3: Voicemail JSONL store

**Files:**
- Create: `internal/domain/voicemail.go`
- Create: `internal/domain/voicemail_test.go`

**Interfaces:**
- Produces: `domain.VoicemailEntry`, `domain.VoicemailStore` with `Leave(*Call) error`, `OldestUnheard() (*VoicemailEntry, error)`, `MarkHeard(id string) error`, `Count() (int, error)`

- [ ] **Step 1: Write failing tests**

```go
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
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/domain/ -run Voicemail -v
```

- [ ] **Step 3: Implement store**

`MarkHeard` rewrites the JSONL file omitting that entry (delete-on-heard). Each line is one JSON object:

```go
type VoicemailEntry struct {
	CallID      string      `json:"call_id"`
	Prompt      string      `json:"prompt"`
	Mode        Mode        `json:"mode"`
	MaxDigits   int         `json:"max_digits"`
	FocusTarget FocusTarget `json:"focus_target"`
	CreatedAt   time.Time   `json:"created_at"`
}
```

Implement `OpenVoicemailStore`, `Leave` (append line), `OldestUnheard` (first line), `MarkHeard` (rewrite file without that `call_id`), `Count`.

- [ ] **Step 4: Run — expect PASS**

```bash
go test ./internal/domain/ -run Voicemail -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/domain/voicemail.go internal/domain/voicemail_test.go
git commit -m "feat: persist unheard Voicemail as JSONL"
```

---

### Task 4: Dialplan Feature Code matching

**Files:**
- Create: `internal/domain/dialplan.go`
- Create: `internal/domain/dialplan_test.go`

**Interfaces:**
- Produces: `domain.Dialplan` with `MatchIdle(digits string) IdleAction`, constants `IdleNone`, `IdleVoicemail`, `IdleError`

- [ ] **Step 1: Write failing tests**

```go
func TestDialplan_VoicemailCode(t *testing.T) {
	d := domain.NewDialplan("*1")
	if d.MatchIdle("*1") != domain.IdleVoicemail {
		t.Fatal("want voicemail")
	}
	if d.MatchIdle("1") != domain.IdleError {
		t.Fatal("want error for unknown")
	}
	if d.MatchIdle("") != domain.IdleNone {
		t.Fatal("want none for empty")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/domain/ -run Dialplan -v
```

- [ ] **Step 3: Implement**

```go
package domain

type IdleAction int

const (
	IdleNone IdleAction = iota
	IdleVoicemail
	IdleError
)

type Dialplan struct {
	VoicemailCode string
}

func NewDialplan(voicemailCode string) Dialplan {
	if voicemailCode == "" {
		voicemailCode = "*1"
	}
	return Dialplan{VoicemailCode: voicemailCode}
}

func (d Dialplan) MatchIdle(digits string) IdleAction {
	if digits == "" {
		return IdleNone
	}
	if digits == d.VoicemailCode {
		return IdleVoicemail
	}
	return IdleError
}
```

- [ ] **Step 4: Run — expect PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/domain/dialplan.go internal/domain/dialplan_test.go
git commit -m "feat: add idle Dialplan Feature Code matching"
```

---

### Task 5: Keystroke recipe + Focuser interface

**Files:**
- Create: `internal/focus/recipe.go`
- Create: `internal/focus/recipe_test.go`
- Create: `internal/focus/focus.go`
- Create: `internal/focus/fake.go`

**Interfaces:**
- Produces: `focus.KeysForDigits(digits string) []string` → e.g. `["1","Enter"]`
- Produces: `focus.Focuser` interface: `Focus(ctx, domain.FocusTarget) error`, `Type(ctx, keys []string) error`
- Produces: `focus.Fake` recording Focus/Type calls

- [ ] **Step 1: Write failing recipe test**

```go
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
}
```

- [ ] **Step 2: Implement recipe + interfaces**

```go
package focus

func KeysForDigits(digits string) []string {
	keys := make([]string, 0, len(digits)+1)
	for _, r := range digits {
		keys = append(keys, string(r))
	}
	keys = append(keys, "Enter")
	return keys
}
```

```go
package focus

import (
	"context"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

type Focuser interface {
	Focus(ctx context.Context, target domain.FocusTarget) error
	Type(ctx context.Context, keys []string) error
}
```

```go
package focus

import (
	"context"
	"sync"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

type Fake struct {
	mu     sync.Mutex
	Focuses []domain.FocusTarget
	Typed   [][]string
	FailFocus bool
}

func (f *Fake) Focus(ctx context.Context, target domain.FocusTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Focuses = append(f.Focuses, target)
	if f.FailFocus {
		return context.Canceled
	}
	return nil
}

func (f *Fake) Type(ctx context.Context, keys []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := append([]string(nil), keys...)
	f.Typed = append(f.Typed, cp)
	return nil
}
```

- [ ] **Step 3: Run — expect PASS**

```bash
go test ./internal/focus/ -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/focus/
git commit -m "feat: add keystroke recipe and Focuser interface"
```

---

### Task 6: Line + Synthesizer interfaces and Fake Line

**Files:**
- Create: `internal/line/line.go`
- Create: `internal/line/fake.go`
- Create: `internal/line/fake_test.go`
- Create: `internal/synth/synth.go`
- Create: `internal/synth/fake.go`

**Interfaces:**
- Produces: `line.Line` — `Ring(ctx) error`, `WaitAnswer(ctx) error`, `Play(ctx, audio []byte) error`, `CollectDigits(ctx, max int) (string, error)`, `Hangup(ctx) error`, `OnIdleDigits(handler func(digits string))`
- Produces: `line.Fake` scriptable: auto-answer after `AnswerAfter`, return `Digits`, or never answer until ctx done
- Produces: `synth.Synthesizer` — `Synthesize(ctx, prompt string) ([]byte, error)`
- Produces: `synth.Fake` returns `[]byte(prompt)` or error if `Fail`

- [ ] **Step 1: Write Fake Line test**

```go
func TestFakeLine_AnswerAndDigits(t *testing.T) {
	ctx := context.Background()
	l := line.NewFake(line.FakeScript{AnswerAfter: 10 * time.Millisecond, Digits: "1"})
	if err := l.Ring(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.WaitAnswer(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.Play(ctx, []byte("audio")); err != nil {
		t.Fatal(err)
	}
	d, err := l.CollectDigits(ctx, 1)
	if err != nil || d != "1" {
		t.Fatalf("digits=%q err=%v", d, err)
	}
}
```

- [ ] **Step 2: Implement interfaces + Fake**

Keep implementations small and synchronous aside from `AnswerAfter` timer. `WaitAnswer` returns `context.DeadlineExceeded` / `ctx.Err()` if script has `NoAnswer: true`.

- [ ] **Step 3: Run — expect PASS**

```bash
go test ./internal/line/ ./internal/synth/ -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/line/ internal/synth/
git commit -m "feat: add Line and Synthesizer interfaces with fakes"
```

---

### Task 7: Call runner (core orchestration)

**Files:**
- Create: `internal/runner/runner.go`
- Create: `internal/runner/runner_test.go`
- Create: `internal/config/config.go`

**Interfaces:**
- Consumes: Queue, Line, Synthesizer, Focuser, VoicemailStore, Dialplan
- Produces: `runner.Runner` with `Start(ctx)`, `Enqueue` via shared Queue (HTTP will use Queue directly); runner loop `processNext`
- Config: `RingTimeout time.Duration` default 90s, `LineRecoverWait` default 3s

- [ ] **Step 1: Write failing integration-style tests**

```go
func TestRunner_CompletedDeliversKeystrokes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := domain.NewQueue()
	vmPath := filepath.Join(t.TempDir(), "vm.jsonl")
	vm, _ := domain.OpenVoicemailStore(vmPath)
	foc := &focus.Fake{}
	ln := line.NewFake(line.FakeScript{AnswerAfter: time.Millisecond, Digits: "1"})
	syn := &synth.Fake{}
	r := runner.New(runner.Deps{
		Queue: q, Line: ln, Synth: syn, Focus: foc, Voicemail: vm,
		Dialplan: domain.NewDialplan("*1"),
		RingTimeout: time.Second,
	})
	go r.Start(ctx)
	c, _ := domain.NewCall("approve?", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	_ = q.Enqueue(c)
	waitStatus(t, q, c.ID, domain.StatusCompleted, 2*time.Second)
	if len(foc.Typed) != 1 || foc.Typed[0][0] != "1" || foc.Typed[0][1] != "Enter" {
		t.Fatalf("typed=%v", foc.Typed)
	}
}

func TestRunner_NoAnswerLeavesVoicemail(t *testing.T) {
	// FakeScript{NoAnswer: true}, RingTimeout: 50ms
	// expect StatusVoicemail and store.Count()==1
}

func TestRunner_FocusFailStillCompletes(t *testing.T) {
	// foc.FailFocus=true; expect StatusCompleted and DeliveryFocusMissed
}
```

Helper `waitStatus` polls `q.Get` until status matches or timeout.

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/runner/ -v
```

- [ ] **Step 3: Implement runner loop**

Algorithm per Call:

1. Set `ringing`; `synth.Synthesize`; on error → `failed`
2. If Line not ready: wait up to `LineRecoverWait`, else `failed`
3. `Ring` + `WaitAnswer` with `RingTimeout` ctx
4. Timeout → `Leave` voicemail, status `voicemail`
5. Answer → `Focus` (on err set delivery focus_missed), `Play`, `CollectDigits`, `Type(KeysForDigits)`, `Hangup`, status `completed`

Also handle idle: when no active Call, if Line reports idle digits via callback, run Dialplan; on `IdleVoicemail`, play oldest (synthesize prompt), collect digits, focus that entry’s target, `MarkHeard`.

- [ ] **Step 4: Run — expect PASS**

- [ ] **Step 5: Commit**

```bash
git add internal/runner/ internal/config/
git commit -m "feat: add Call runner with Voicemail and focus delivery"
```

---

### Task 8: Config + HTTP API Surface

**Files:**
- Modify: `internal/config/config.go` (bind addr, data dir, timeouts, feature code, line backend)
- Create: `internal/api/server.go`
- Create: `internal/api/server_test.go`
- Create: `internal/app/app.go`

**Interfaces:**
- Produces: HTTP on `127.0.0.1:8741` (configurable)
  - `POST /v1/calls` body `{prompt, mode, max_digits, focus_target}` → `201 {id}`
  - `GET /v1/calls/{id}` → Call JSON
  - `GET /v1/queue` → `{depth, calls:[...]}`
  - `GET /v1/voicemail` → `{count, oldest?:...}`
  - `GET /v1/health` → `{ok:true, line: "up"|"down"}`

- [ ] **Step 1: Write API test with Fake Line runner**

Use `net/http/httptest` against `api.NewServer(app)`. POST call, poll GET until `completed` with Fake auto-answer.

- [ ] **Step 2: Implement server + App wiring**

`app.App` owns Queue, Runner, stores, starts runner goroutine.

- [ ] **Step 3: Run — expect PASS**

```bash
go test ./internal/api/ -v
```

- [ ] **Step 4: Commit**

```bash
git add internal/api/ internal/app/ internal/config/
git commit -m "feat: add localhost HTTP API for enqueue and status"
```

---

### Task 9: CLI Surface

**Files:**
- Modify: `cmd/acc/main.go`
- Create: `internal/cli/root.go` (optional; may live in cmd)

**Interfaces:**
- `acc serve` — start App + HTTP (+ tray later)
- `acc call --prompt "..." --app Cursor --title "..." [--digits 1]`
- `acc status`
- `acc voicemail`

CLI talks to HTTP (`ACC_URL` default `http://127.0.0.1:8741`).

- [ ] **Step 1: Add cobra**

```bash
go get github.com/spf13/cobra
```

- [ ] **Step 2: Implement commands; smoke-test with `go test` starting server in-process or document manual:**

```bash
go run ./cmd/acc serve &
go run ./cmd/acc call --prompt "approve?" --app Cursor
go run ./cmd/acc status
```

Add `internal/cli/cli_test.go` that starts App with Fake Line on random port and shells out to call logic via `api` client package.

- [ ] **Step 3: Commit**

```bash
git add cmd/acc/ internal/cli/
git commit -m "feat: add acc CLI (serve, call, status, voicemail)"
```

---

### Task 10: MCP Surface

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/server_test.go`

**Interfaces:**
- Tools: `place_call` (same fields as POST; returns `id`), `get_call`, `get_queue`, `get_voicemail`
- No blocking wait-for-reply tool
- Started from `acc serve` on stdio **or** as additional session alongside HTTP — prefer: MCP over stdio when `acc mcp` subcommand; `acc serve` documents HTTP for hooks. Spec asked MCP in v1: implement `acc mcp` stdio server sharing App connection via HTTP client to running daemon **or** embed App. **Embed:** `acc serve --mcp` optional; simpler: `acc mcp` requires daemon HTTP and is a thin MCP↔HTTP proxy so one App instance owns the Line.

**Decision for implementer:** `acc mcp` is an MCP stdio proxy to the local HTTP API (daemon must be running). Tools call HTTP.

- [ ] **Step 1: Add MCP Go SDK dependency (current official module at implement time)**

- [ ] **Step 2: Write test that registers tools and invokes `place_call` against httptest API**

- [ ] **Step 3: Implement and commit**

```bash
git commit -m "feat: add MCP stdio proxy tools over local HTTP"
```

---

### Task 11: Virtual Line (manual testing)

**Files:**
- Create: `internal/line/virtual.go`
- Modify: `internal/config/config.go` — `line_backend=virtual|fake|sip`
- Modify: `internal/app/app.go` — construct Virtual Line

**Behavior:**
- On `Ring`, play system beep / log “RINGING”
- `WaitAnswer`: wait for Enter key on stdin **or** a small OS notification “press F9 to answer” — **v1:** listen on a local UDP/TCP control port `127.0.0.1:8742` for commands `ANSWER`, `DTMF 1`, `HANGUP`, and also stdin when running in foreground
- `Play`: write WAV to default speaker via `beep` or OS (`afplay` on macOS for temp file)
- `CollectDigits`: wait for DTMF messages / keyboard digits

- [ ] **Step 1: Manual checklist in README (no CI audio)**

- [ ] **Step 2: Implement Virtual Line using temp WAV + `afplay`/`ffplay`/`powershell` media; keyboard digit collection when `ACC_VIRTUAL_STDIN=1`

- [ ] **Step 3: Commit**

```bash
git commit -m "feat: add Virtual Line for speaker/keyboard testing"
```

---

### Task 12: Status Tray

**Files:**
- Create: `internal/tray/tray.go`
- Modify: `cmd/acc` / `app` — `acc serve` starts tray on desktop OS

**Behavior (getlantern/systray):**
- Title/tooltip: `acc · queue N · vm M · line up|down`
- Menu: Status (disabled text), Voicemails count, Quit → cancel App context

- [ ] **Step 1: Add dependency**

```bash
go get github.com/getlantern/systray
```

- [ ] **Step 2: Implement tray polling App state every 1s**

- [ ] **Step 3: Manual verify on macOS/Windows; commit**

```bash
git commit -m "feat: add Status Tray for queue and Line health"
```

---

### Task 13: OS Synthesizer

**Files:**
- Create: `internal/synth/os.go`
- Modify: app wiring — use OS synth when not in tests

**Behavior:**
- macOS: `say -o outfile.aiff` then convert/read bytes (or `say` to AIFF and load)
- Windows: PowerShell `Add-Type` System.Speech or `System.Speech.Synthesis` to WAV file
- Linux: `espeak-ng -w outfile.wav`
- Return WAV/AIFF bytes for Line.Play

- [ ] **Step 1: Unit-test with build tag `manual` skipped in CI; fake remains default in runner tests**

- [ ] **Step 2: Implement `synth.NewOS()` selecting by `runtime.GOOS`

- [ ] **Step 3: Commit**

```bash
git commit -m "feat: add OS TTS Synthesizer (say/SAPI/espeak-ng)"
```

---

### Task 14: OS Focuser (macOS primary)

**Files:**
- Create: `internal/focus/os_darwin.go`
- Create: `internal/focus/os_other.go`
- Create: `internal/focus/os_windows.go` (best-effort)

**Behavior (darwin):**
- Use Accessibility APIs via `osascript` or cgo:
  - Find process/window where name contains `TitleContains` and app name matches `App`
  - Activate application
  - Keystroke digits via System Events
- On failure return error (runner sets `focus_missed`)

- [ ] **Step 1: Document macOS Accessibility permission in README**

- [ ] **Step 2: Implement darwin Focuser; other platforms return `errors.New("focuser not supported")` or best-effort**

- [ ] **Step 3: Manual test with Fake/Virtual Call; commit**

```bash
git commit -m "feat: add macOS Accessibility Focuser for Focus Target"
```

---

### Task 15: Embedded SIP Line (PAP2T)

**Files:**
- Create: `internal/line/sip/line.go`
- Create: `internal/line/sip/rtp.go` (minimal)
- Modify: config — SIP listen addr, realm, user/pass for ATA, extension to ring

**Behavior:**
- Daemon acts as SIP registrar/UA on LAN (e.g. `0.0.0.0:5060` — note: SIP must bind beyond localhost for PAP2T; **HTTP stays localhost**; document firewall)
- PAP2T registers as SIP client
- `Ring` sends INVITE to registered contact
- Detect answer; play RTP audio (μ-law/alaw from WAV)
- Collect RFC2833 or in-band DTMF
- Hangup BYE

**Library:** `github.com/emiago/sipgo` (+ minimal RTP)

- [ ] **Step 1: Spike README section “PAP2T settings” (proxy IP = PC, extension, codec G.711)**

- [ ] **Step 2: Implement register + invite + RTP playback + DTMF; test with Virtual fallback if no hardware**

- [ ] **Step 3: Manual PAP2T test checklist; commit**

```bash
git commit -m "feat: add embedded SIP Line for PAP2T registration"
```

---

### Task 16: Phone Mic (macOS + BlackHole)

**Files:**
- Create: `internal/phonemic/phonemic.go`
- Create: `internal/phonemic/macos.go`
- Create: `internal/phonemic/stub.go`
- Modify: config `phone_mic_enabled`, `phone_mic_device=BlackHole 2ch`
- Modify: Line/SIP/Virtual to optionally tee RX PCM to PhoneMic when off-hook

**Behavior:**
- Interface `Start()`, `WritePCM(pcm []byte)`, `Stop()`
- macOS: open output to BlackHole device (Core Audio via cgo or `sox`/`ffmpeg` to device) — document “Install BlackHole; select as mic in Cursor”
- stub on !darwin

- [ ] **Step 1: README install steps for BlackHole**

- [ ] **Step 2: Implement tee from Virtual/SIP RX path**

- [ ] **Step 3: Manual verify Cursor dictation; commit**

```bash
git commit -m "feat: tee Line audio to macOS Phone Mic (BlackHole)"
```

---

### Task 17: README + CI

**Files:**
- Create: `README.md`
- Create: `.github/workflows/ci.yml` (or skip if no GH Actions desire — still add `make test`)
- Create: `Makefile` with `test`, `build`

**README must cover:**
- What it is (Agent→You)
- Install, `acc serve`, `acc call` example for Cursor hook
- PAP2T config, Virtual Line, Voicemail `*1`, Accessibility, BlackHole
- Trust model (localhost)

- [ ] **Step 1: Write README**

- [ ] **Step 2: CI runs `go test ./...` (Fake Line only)**

- [ ] **Step 3: Commit**

```bash
git commit -m "docs: add README and CI test workflow"
```

---

## Spec coverage checklist (self-review)

| Spec item | Task(s) |
|---|---|
| Async enqueue, immediate id | 8, 9, 10 |
| FIFO Queue | 2, 7 |
| dtmf only; modes named | 1, 7 |
| Digits + Enter to Focus Target | 5, 7, 14 |
| Voicemail JSONL + `*1` | 3, 4, 7 |
| Idle dial tone only | 4, 7 |
| Fake / Virtual / SIP Lines | 6, 11, 15 |
| HTTP + CLI + MCP | 8, 9, 10 |
| Status Tray | 12 |
| OS TTS | 13 |
| Phone Mic macOS | 16 |
| Localhost HTTP no auth | 8 |
| SIP for PAP2T (LAN bind exception for SIP only) | 15 |
| Testing Fake in CI | 6, 7, 8, 17 |

**Resolved open spec items in this plan:**
- `focus_target` JSON: `app`, `title_contains`, `chat_id`
- SIP library: `sipgo`
- Phone Mic: BlackHole device tee (not a custom kernel driver)
- Defaults: ring 90s, Feature Code `*1`, HTTP `127.0.0.1:8741`
