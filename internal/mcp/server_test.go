package mcp_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/MRsagi/agents-call-center/internal/api"
	"github.com/MRsagi/agents-call-center/internal/app"
	accmcp "github.com/MRsagi/agents-call-center/internal/mcp"
	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/line"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func testAPIServer(t *testing.T) (*httptest.Server, *app.App) {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	a, err := app.NewWithAdapters(cfg, app.Adapters{Line: line.NewFake(line.FakeScript{})})
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(api.NewServer(a).Handler()), a
}

func connectMCPClient(t *testing.T, baseURL string) (*sdkmcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()
	server := accmcp.NewServer(baseURL)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1"}, nil)
	t1, t2 := sdkmcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session, func() { _ = session.Close() }
}

func TestMCPToolsListAndPlaceCall(t *testing.T) {
	srv, _ := testAPIServer(t)
	defer srv.Close()

	session, cleanup := connectMCPClient(t, srv.URL)
	defer cleanup()

	ctx := context.Background()
	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"place_call", "get_call", "get_queue", "get_voicemail"} {
		if !names[want] {
			t.Fatalf("missing tool %q; got %v", want, names)
		}
	}

	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "place_call",
		Arguments: map[string]any{
			"prompt": "approve?",
			"app":    "Cursor",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("place_call error: %+v", res)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID == "" {
		t.Fatal("empty id")
	}

	getRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "get_call",
		Arguments: map[string]any{"id": out.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if getRes.IsError {
		t.Fatalf("get_call error: %+v", getRes)
	}
}
