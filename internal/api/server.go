package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/domain"
)

type Server struct {
	app *app.App
	mux *http.ServeMux
}

func NewServer(a *app.App) *Server {
	s := &Server{app: a, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /v1/calls", s.handleCreateCall)
	s.mux.HandleFunc("GET /v1/calls/{id}", s.handleGetCall)
	s.mux.HandleFunc("GET /v1/queue", s.handleQueue)
	s.mux.HandleFunc("GET /v1/voicemail", s.handleVoicemail)
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
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
	Prompt      string              `json:"prompt"`
	Mode        domain.Mode         `json:"mode"`
	MaxDigits   int                 `json:"max_digits"`
	FocusTarget domain.FocusTarget  `json:"focus_target"`
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
	Count  int                     `json:"count"`
	Oldest *domain.VoicemailEntry  `json:"oldest,omitempty"`
}

func (s *Server) handleVoicemail(w http.ResponseWriter, r *http.Request) {
	count, err := s.app.Voicemail.Count()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	resp := voicemailResponse{Count: count}
	if count > 0 {
		oldest, err := s.app.Voicemail.OldestUnheard()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		resp.Oldest = oldest
	}
	writeJSON(w, http.StatusOK, resp)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
