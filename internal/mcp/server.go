package mcp

import (
	"context"
	"fmt"

	"github.com/MRsagi/agents-call-center/internal/apiclient"
	"github.com/MRsagi/agents-call-center/internal/domain"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Proxy forwards MCP tool calls to the local acc HTTP API.
type Proxy struct {
	API *apiclient.Client
}

func NewProxy(baseURL string) *Proxy {
	return &Proxy{API: apiclient.NewClient(baseURL)}
}

type placeCallArgs struct {
	Prompt    string `json:"prompt" jsonschema:"Prompt read to the agent"`
	App       string `json:"app" jsonschema:"Focus target application name"`
	Title     string `json:"title,omitempty" jsonschema:"Optional window title substring"`
	MaxDigits int    `json:"max_digits,omitempty" jsonschema:"Max DTMF digits to collect"`
}

type placeCallOut struct {
	ID string `json:"id"`
}

type getCallArgs struct {
	ID string `json:"id" jsonschema:"Call id returned from place_call"`
}

type emptyArgs struct{}

func RegisterTools(s *sdkmcp.Server, p *Proxy) {
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "place_call",
		Description: "Enqueue a DTMF call on the local line; returns the call id.",
	}, p.placeCall)
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_call",
		Description: "Fetch call status and collected digits by id.",
	}, p.getCall)
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_queue",
		Description: "Snapshot the outbound call queue.",
	}, p.getQueue)
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_voicemail",
		Description: "Voicemail box summary (count and oldest unheard entry).",
	}, p.getVoicemail)
}

func NewServer(baseURL string) *sdkmcp.Server {
	s := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "agents-call-center",
		Version: "1",
	}, nil)
	RegisterTools(s, NewProxy(baseURL))
	return s
}

func Run(ctx context.Context, baseURL string) error {
	return NewServer(baseURL).Run(ctx, &sdkmcp.StdioTransport{})
}

func (p *Proxy) placeCall(_ context.Context, _ *sdkmcp.CallToolRequest, args placeCallArgs) (*sdkmcp.CallToolResult, placeCallOut, error) {
	id, err := p.API.CreateCall(apiclient.CreateCallInput{
		Prompt:    args.Prompt,
		App:       args.App,
		Title:     args.Title,
		MaxDigits: args.MaxDigits,
	})
	if err != nil {
		return toolError(err), placeCallOut{}, nil
	}
	return nil, placeCallOut{ID: id}, nil
}

func (p *Proxy) getCall(_ context.Context, _ *sdkmcp.CallToolRequest, args getCallArgs) (*sdkmcp.CallToolResult, domain.Call, error) {
	if args.ID == "" {
		return toolError(fmt.Errorf("id required")), domain.Call{}, nil
	}
	c, err := p.API.GetCall(args.ID)
	if err != nil {
		return toolError(err), domain.Call{}, nil
	}
	return nil, c, nil
}

func (p *Proxy) getQueue(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyArgs) (*sdkmcp.CallToolResult, apiclient.QueueSummary, error) {
	q, err := p.API.Queue()
	if err != nil {
		return toolError(err), apiclient.QueueSummary{}, nil
	}
	return nil, q, nil
}

func (p *Proxy) getVoicemail(_ context.Context, _ *sdkmcp.CallToolRequest, _ emptyArgs) (*sdkmcp.CallToolResult, apiclient.VoicemailSummary, error) {
	v, err := p.API.Voicemail()
	if err != nil {
		return toolError(err), apiclient.VoicemailSummary{}, nil
	}
	return nil, v, nil
}

func toolError(err error) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: err.Error()},
		},
	}
}
