package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/MRsagi/agents-call-center/internal/api/console"
	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/domain"
)

type Server struct {
	app       *app.App
	mux       *http.ServeMux
	OnRestart func() // set by serve loop (Task 9)
}

func NewServer(a *app.App) *Server {
	s := &Server{app: a, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /v1/calls", s.handleCreateCall)
	s.mux.HandleFunc("GET /v1/calls/{id}", s.handleGetCall)
	s.mux.HandleFunc("GET /v1/queue", s.handleQueue)
	s.mux.HandleFunc("GET /v1/voicemail", s.handleVoicemail)
	s.mux.HandleFunc("DELETE /v1/voicemail/{id}", s.handleDeleteVoicemailID)
	s.mux.HandleFunc("DELETE /v1/voicemail", s.handleDeleteVoicemailAll)
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /v1/status", s.handleStatus)
	s.mux.HandleFunc("GET /v1/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /v1/config", s.handlePutConfig)
	s.mux.HandleFunc("GET /v1/events", s.handleEvents)

	sub, err := fs.Sub(console.FS, ".")
	if err != nil {
		panic("console embed: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	s.mux.Handle("GET /console/", http.StripPrefix("/console/", fileServer))
	s.mux.Handle("GET /console", http.RedirectHandler("/console/", http.StatusFound))
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) ValidateBind() error {
	return validateLocalhostBind(s.app.Config.HTTPBind)
}

func (s *Server) ListenAndServe() error {
	addr := s.app.Config.HTTPBind
	if err := validateLocalhostBind(addr); err != nil {
		return err
	}
	return http.ListenAndServe(addr, s.mux)
}

func validateLocalhostBind(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host != "127.0.0.1" && host != "localhost" {
		return errors.New("HTTP bind must be localhost only")
	}
	return nil
}

type createCallRequest struct {
	Prompt      string             `json:"prompt"`
	Mode        domain.Mode        `json:"mode"`
	MaxDigits   int                `json:"max_digits"`
	FocusTarget domain.FocusTarget `json:"focus_target"`
}

type createCallResponse struct {
	ID string `json:"id"`
}

func (s *Server) handleCreateCall(w http.ResponseWriter, r *http.Request) {
	var req createCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	c, err := domain.NewCall(req.Prompt, req.Mode, req.FocusTarget, req.MaxDigits)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.app.Queue.Enqueue(c); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, createCallResponse{ID: c.ID})
}

func (s *Server) handleGetCall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := s.app.Queue.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "call not found"})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type queueResponse struct {
	Depth int            `json:"depth"`
	Calls []*domain.Call `json:"calls"`
}

func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, queueResponse{
		Depth: s.app.Queue.Len(),
		Calls: s.app.Queue.Snapshot(),
	})
}

type voicemailResponse struct {
	Count   int                     `json:"count"`
	Oldest  *domain.VoicemailEntry  `json:"oldest,omitempty"`
	Entries []domain.VoicemailEntry `json:"entries"`
}

func (s *Server) handleVoicemail(w http.ResponseWriter, r *http.Request) {
	entries, err := s.app.Voicemail.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []domain.VoicemailEntry{}
	}
	resp := voicemailResponse{
		Count:   len(entries),
		Entries: entries,
	}
	if len(entries) > 0 {
		e := entries[0]
		resp.Oldest = &e
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteVoicemailID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.app.Voicemail.MarkHeard(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.app.Events != nil {
		s.app.Events.Publish(domain.Event{Type: domain.EventVoicemailChanged})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteVoicemailAll(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Voicemail.DeleteAll(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.app.Events != nil {
		s.app.Events.Publish(domain.Event{Type: domain.EventVoicemailChanged})
	}
	w.WriteHeader(http.StatusNoContent)
}

type healthResponse struct {
	OK   bool   `json:"ok"`
	Line string `json:"line"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	line := "down"
	if s.app.LineUp() {
		line = "up"
	}
	writeJSON(w, http.StatusOK, healthResponse{OK: true, Line: line})
}

type statusResponse struct {
	Line             string         `json:"line"`
	QueueDepth       int            `json:"queue_depth"`
	ActiveCall       *domain.Call   `json:"active_call"`
	IdleDictating    bool           `json:"idle_dictating"`
	Recent           []*domain.Call `json:"recent"`
	VoicemailCount   int            `json:"voicemail_count"`
	DictationWarning bool           `json:"dictation_warning,omitempty"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	line := "down"
	if s.app.LineUp() {
		line = "up"
	}
	vmCount := 0
	if s.app.Voicemail != nil {
		if n, err := s.app.Voicemail.Count(); err == nil {
			vmCount = n
		}
	}
	recent := []*domain.Call{}
	if s.app.Recent != nil {
		recent = s.app.Recent.Snapshot()
		if recent == nil {
			recent = []*domain.Call{}
		}
	}
	resp := statusResponse{
		Line:           line,
		QueueDepth:     s.app.Queue.Len(),
		ActiveCall:     s.app.Queue.ActiveCall(),
		IdleDictating:  s.app.IdleDictating(),
		Recent:         recent,
		VoicemailCount: vmCount,
	}
	if s.app.DictationWarning() {
		resp.DictationWarning = true
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, overrides, err := config.LoadEffective(s.app.Config.DataDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if overrides == nil {
		overrides = map[string]bool{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config":        config.AsFileDTO(cfg),
		"env_overrides": overrides,
	})
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	cfg, err := config.FromFileJSON(data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw["data_dir"]; !ok {
		cfg.DataDir = s.app.Config.DataDir
	}
	if err := config.Validate(cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(s.app.Config.DataDir, "config.json")
	if err := config.SaveFile(path, cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.OnRestart != nil {
		s.OnRestart()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"config": config.AsFileDTO(cfg),
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	if s.app.Events == nil {
		<-r.Context().Done()
		return
	}
	ch := s.app.Events.Subscribe(r.Context())
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, err := json.Marshal(ev.Payload)
			if err != nil {
				payload = []byte("null")
			}
			at := ev.At
			if at.IsZero() {
				at = time.Now().UTC()
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\nid: %d\n\n", ev.Type, payload, at.UnixNano())
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
