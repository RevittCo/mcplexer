package control

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/revitteth/mcplexer/internal/gateway"
	"github.com/revitteth/mcplexer/internal/store"
)

func TestServerInitialize(t *testing.T) {
	srv := New(newTestDB(t))
	var in, out bytes.Buffer

	writeReq(&in, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
	})

	if err := srv.run(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}

	responses := readResponses(t, out.Bytes())
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	var result gateway.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.ServerInfo.Name != "mcplexer-control" {
		t.Fatalf("server name = %q", result.ServerInfo.Name)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Fatalf("protocol version = %q", result.ProtocolVersion)
	}
}

func TestServerPing(t *testing.T) {
	srv := New(newTestDB(t))
	var in, out bytes.Buffer

	writeReq(&in, 1, "ping", nil)

	if err := srv.run(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}

	responses := readResponses(t, out.Bytes())
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("unexpected error: %s", responses[0].Error.Message)
	}
}

func TestServerToolsList(t *testing.T) {
	t.Run("read-write", func(t *testing.T) {
		srv := New(newTestDB(t), false)
		var in, out bytes.Buffer
		writeReq(&in, 1, "tools/list", nil)

		if err := srv.run(context.Background(), &in, &out); err != nil {
			t.Fatal(err)
		}

		var result struct {
			Tools []gateway.Tool `json:"tools"`
		}
		if err := json.Unmarshal(readResponses(t, out.Bytes())[0].Result, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Tools) != 19 {
			t.Fatalf("got %d tools, want 19", len(result.Tools))
		}

		names := make(map[string]bool)
		for _, tool := range result.Tools {
			names[tool.Name] = true
		}
		for _, want := range []string{"list_servers", "status", "create_server", "query_audit"} {
			if !names[want] {
				t.Errorf("missing tool: %s", want)
			}
		}
	})

	t.Run("read-only", func(t *testing.T) {
		srv := New(newTestDB(t)) // default: read-only
		var in, out bytes.Buffer
		writeReq(&in, 1, "tools/list", nil)

		if err := srv.run(context.Background(), &in, &out); err != nil {
			t.Fatal(err)
		}

		var result struct {
			Tools []gateway.Tool `json:"tools"`
		}
		if err := json.Unmarshal(readResponses(t, out.Bytes())[0].Result, &result); err != nil {
			t.Fatal(err)
		}
		// 19 total - 11 admin tools = 8 read-only tools.
		if len(result.Tools) != 8 {
			t.Fatalf("got %d tools, want 8 (read-only)", len(result.Tools))
		}

		// Admin tools should be absent.
		names := make(map[string]bool)
		for _, tool := range result.Tools {
			names[tool.Name] = true
		}
		if names["create_server"] {
			t.Error("create_server should not be listed in read-only mode")
		}
	})
}

func TestServerToolsCall_ListServers(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedServer(t, db)

	srv := New(db)
	var in, out bytes.Buffer
	writeReq(&in, 1, "tools/call", map[string]any{"name": "list_servers"})

	if err := srv.run(ctx, &in, &out); err != nil {
		t.Fatal(err)
	}

	resp := readResponses(t, out.Bytes())[0]
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %s", resp.Error.Message)
	}

	text, isErr := parseToolResult(t, resp.Result)
	if isErr {
		t.Fatalf("tool error: %s", text)
	}

	var servers []store.DownstreamServer
	if err := json.Unmarshal([]byte(text), &servers); err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if servers[0].Name != "test-server" {
		t.Fatalf("server name = %q, want %q", servers[0].Name, "test-server")
	}
}

func TestServerToolsCall_Status(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	seedServer(t, db)

	srv := New(db)
	var in, out bytes.Buffer
	writeReq(&in, 1, "tools/call", map[string]any{"name": "status"})

	if err := srv.run(ctx, &in, &out); err != nil {
		t.Fatal(err)
	}

	resp := readResponses(t, out.Bytes())[0]
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %s", resp.Error.Message)
	}

	text, isErr := parseToolResult(t, resp.Result)
	if isErr {
		t.Fatalf("tool error: %s", text)
	}

	var status map[string]int
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatal(err)
	}
	if status["downstream_servers"] != 1 {
		t.Fatalf("downstream_servers = %d, want 1", status["downstream_servers"])
	}
	if status["workspaces"] != 0 {
		t.Fatalf("workspaces = %d, want 0", status["workspaces"])
	}
}

func TestServerToolsCall_CreateServer(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	srv := New(db, false) // read-write mode

	var in, out bytes.Buffer
	writeReq(&in, 1, "tools/call", map[string]any{
		"name": "create_server",
		"arguments": map[string]any{
			"name":           "new-server",
			"command":        "python",
			"tool_namespace": "py",
		},
	})

	if err := srv.run(ctx, &in, &out); err != nil {
		t.Fatal(err)
	}

	resp := readResponses(t, out.Bytes())[0]
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %s", resp.Error.Message)
	}

	text, isErr := parseToolResult(t, resp.Result)
	if isErr {
		t.Fatalf("tool error: %s", text)
	}

	// Verify server was created in DB.
	servers, err := db.ListDownstreamServers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if servers[0].Name != "new-server" {
		t.Fatalf("name = %q, want %q", servers[0].Name, "new-server")
	}
}

func TestServerNotification(t *testing.T) {
	srv := New(newTestDB(t))
	var in, out bytes.Buffer

	writeNotification(&in, "notifications/initialized")
	writeReq(&in, 1, "ping", nil)

	if err := srv.run(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}

	responses := readResponses(t, out.Bytes())
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1 (notification should not produce response)", len(responses))
	}
}

func TestServerUnknownMethod(t *testing.T) {
	srv := New(newTestDB(t))
	var in, out bytes.Buffer

	writeReq(&in, 1, "unknown/method", nil)

	if err := srv.run(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}

	responses := readResponses(t, out.Bytes())
	if len(responses) != 1 {
		t.Fatalf("got %d responses", len(responses))
	}
	if responses[0].Error == nil {
		t.Fatal("expected error response")
	}
	if responses[0].Error.Code != gateway.CodeMethodNotFound {
		t.Fatalf("error code = %d, want %d", responses[0].Error.Code, gateway.CodeMethodNotFound)
	}
}

func TestServerInvalidJSON(t *testing.T) {
	srv := New(newTestDB(t))
	var out bytes.Buffer
	in := bytes.NewBufferString("this is not json\n")

	if err := srv.run(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}

	responses := readResponses(t, out.Bytes())
	if len(responses) != 1 {
		t.Fatalf("got %d responses", len(responses))
	}
	if responses[0].Error == nil {
		t.Fatal("expected parse error")
	}
	if responses[0].Error.Code != gateway.CodeParseError {
		t.Fatalf("error code = %d, want %d", responses[0].Error.Code, gateway.CodeParseError)
	}
}

func TestServerUnknownTool(t *testing.T) {
	srv := New(newTestDB(t))
	var in, out bytes.Buffer

	writeReq(&in, 1, "tools/call", map[string]any{"name": "nonexistent_tool"})

	if err := srv.run(context.Background(), &in, &out); err != nil {
		t.Fatal(err)
	}

	responses := readResponses(t, out.Bytes())
	if responses[0].Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if responses[0].Error.Code != gateway.CodeMethodNotFound {
		t.Fatalf("error code = %d", responses[0].Error.Code)
	}
}
