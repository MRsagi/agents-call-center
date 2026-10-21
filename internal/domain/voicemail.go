package domain

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type VoicemailEntry struct {
	CallID      string      `json:"call_id"`
	Prompt      string      `json:"prompt"`
	Mode        Mode        `json:"mode"`
	MaxDigits   int         `json:"max_digits"`
	FocusTarget FocusTarget `json:"focus_target"`
	CreatedAt   time.Time   `json:"created_at"`
}

type VoicemailStore interface {
	Leave(*Call) error
	OldestUnheard() (*VoicemailEntry, error)
	MarkHeard(id string) error
	List() ([]VoicemailEntry, error)
	DeleteAll() error
	Count() (int, error)
}

type jsonlVoicemailStore struct {
	path string
	mu   sync.Mutex
}

func OpenVoicemailStore(path string) (VoicemailStore, error) {
	return &jsonlVoicemailStore{path: path}, nil
}

func (s *jsonlVoicemailStore) Leave(c *Call) error {
	if c == nil {
		return errors.New("call required")
	}
	entry := VoicemailEntry{
		CallID:      c.ID,
		Prompt:      c.Prompt,
		Mode:        c.Mode,
		MaxDigits:   c.MaxDigits,
		FocusTarget: c.FocusTarget,
		CreatedAt:   c.CreatedAt,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func (s *jsonlVoicemailStore) readAll() ([]VoicemailEntry, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var entries []VoicemailEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e VoicemailEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *jsonlVoicemailStore) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

func (s *jsonlVoicemailStore) OldestUnheard() (*VoicemailEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, errors.New("no unheard voicemail")
	}
	e := entries[0]
	return &e, nil
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

func (s *jsonlVoicemailStore) MarkHeard(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.readAll()
	if err != nil {
		return err
	}
	filtered := entries[:0]
	for _, e := range entries {
		if e.CallID != id {
			filtered = append(filtered, e)
		}
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "voicemail-*.jsonl")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}
	for _, e := range filtered {
		line, err := json.Marshal(e)
		if err != nil {
			cleanup()
			return err
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			cleanup()
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if len(filtered) == 0 {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			os.Remove(tmpPath)
			return err
		}
		os.Remove(tmpPath)
		return nil
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename voicemail store: %w", err)
	}
	return nil
}
