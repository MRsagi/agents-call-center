# Task 10 Report: MCP Surface

## Status
Complete.

## Summary
- Added `acc mcp` stdio MCP server (`internal/mcp`) using `github.com/modelcontextprotocol/go-sdk` v1.8.0.
- Tools: `place_call`, `get_call`, `get_queue`, `get_voicemail` — each proxies to the local HTTP API (`ACC_URL` / default `http://127.0.0.1:8741`).
- No blocking wait-for-reply tool.
- Extracted `internal/apiclient` from CLI HTTP client to avoid `cli` ↔ `mcp` import cycle; CLI re-exports types for compatibility.
- Added `GetCall` on the HTTP client for `get_call`.

## Usage
1. Run daemon: `acc serve`
2. Configure MCP host to run: `acc mcp` (optional `--url`)

## Tests
- `internal/mcp`: in-memory MCP session lists four tools; `place_call` + `get_call` against `httptest` API.
- Full suite: `go test ./...` passes.

## Notes
- `go.mod` bumped to Go 1.25.0 (required by MCP SDK).
- MCP server is a thin proxy; daemon must own the Line.
