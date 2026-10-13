package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

const defaultACCURL = "http://127.0.0.1:8741"

func BaseURL() string {
	if v := os.Getenv("ACC_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultACCURL
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = BaseURL()
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

type CreateCallInput struct {
	Prompt    string
	App       string
	Title     string
	MaxDigits int
}

type createCallBody struct {
	Prompt      string             `json:"prompt"`
	Mode        domain.Mode        `json:"mode"`
	MaxDigits   int                `json:"max_digits"`
	FocusTarget domain.FocusTarget `json:"focus_target"`
}

func (c *Client) CreateCall(in CreateCallInput) (string, error) {
	if in.MaxDigits <= 0 {
		in.MaxDigits = 1
	}
	body, err := json.Marshal(createCallBody{
		Prompt:    in.Prompt,
		Mode:      domain.ModeDTMF,
		MaxDigits: in.MaxDigits,
		FocusTarget: domain.FocusTarget{
			App:           in.App,
			TitleContains: in.Title,
		},
	})
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Post(c.BaseURL+"/v1/calls", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("POST /v1/calls: %s", trimAPIError(data, resp.StatusCode))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("empty call id")
	}
	return out.ID, nil
}

func (c *Client) GetCall(id string) (domain.Call, error) {
	var call domain.Call
	path := "/v1/calls/" + url.PathEscape(id)
	if err := c.getJSON(path, &call); err != nil {
		return call, err
	}
	return call, nil
}

type Health struct {
	OK   bool   `json:"ok"`
	Line string `json:"line"`
}

func (c *Client) Health() (Health, error) {
	var h Health
	if err := c.getJSON("/v1/health", &h); err != nil {
		return h, err
	}
	return h, nil
}

type QueueSummary struct {
	Depth int            `json:"depth"`
	Calls []*domain.Call `json:"calls"`
}

func (c *Client) Queue() (QueueSummary, error) {
	var q QueueSummary
	if err := c.getJSON("/v1/queue", &q); err != nil {
		return q, err
	}
	return q, nil
}

type StatusSummary struct {
	Health Health       `json:"health"`
	Queue  QueueSummary `json:"queue"`
}

func (c *Client) Status() (StatusSummary, error) {
	h, err := c.Health()
	if err != nil {
		return StatusSummary{}, err
	}
	q, err := c.Queue()
	if err != nil {
		return StatusSummary{}, err
	}
	return StatusSummary{Health: h, Queue: q}, nil
}

type VoicemailSummary struct {
	Count  int                    `json:"count"`
	Oldest *domain.VoicemailEntry `json:"oldest,omitempty"`
}

func (c *Client) Voicemail() (VoicemailSummary, error) {
	var v VoicemailSummary
	if err := c.getJSON("/v1/voicemail", &v); err != nil {
		return v, err
	}
	return v, nil
}

func (c *Client) getJSON(path string, dest any) error {
	resp, err := c.HTTP.Get(c.BaseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, trimAPIError(data, resp.StatusCode))
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return err
	}
	return nil
}

func trimAPIError(data []byte, code int) string {
	var er struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &er) == nil && er.Error != "" {
		return er.Error
	}
	return fmt.Sprintf("HTTP %d", code)
}
