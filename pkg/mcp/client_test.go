package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMCPServer handles initialize / tools/list / tools/call JSON-RPC calls.
func fakeMCPServer(t *testing.T, listCount *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req struct {
			ID     any             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		require.NoError(t, json.Unmarshal(body, &req))
		write := func(result any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": req.ID, "result": result,
			})
		}
		switch req.Method {
		case "initialize":
			write(map[string]any{
				"protocolVersion": "2025-03-26",
				"capabilities":    map[string]any{},
				"serverInfo":      map[string]any{"name": "fake", "version": "0"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if listCount != nil {
				atomic.AddInt32(listCount, 1)
			}
			write(map[string]any{"tools": []map[string]any{{
				"name":        "search",
				"description": "search the web",
				"inputSchema": map[string]any{"type": "object"},
			}}})
		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			require.NoError(t, json.Unmarshal(req.Params, &params))
			if params.Name == "fail" {
				write(map[string]any{
					"content": []map[string]any{{"type": "text", "text": "boom"}},
					"isError": true,
				})
				return
			}
			write(map[string]any{
				"content": []map[string]any{{"type": "text", "text": "result for " + params.Name}},
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

func TestListToolsAndCache(t *testing.T) {
	var listCount int32
	srv := fakeMCPServer(t, &listCount)
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	tools, err := c.ListTools(ctx)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "search", tools[0].Name)
	assert.Equal(t, "search the web", tools[0].Description)
	assert.Contains(t, string(tools[0].InputSchema), `"object"`)

	// Second call within TTL must hit the cache.
	_, err = c.ListTools(ctx)
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&listCount))
}

func TestCallTool(t *testing.T) {
	srv := fakeMCPServer(t, nil)
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := c.CallTool(ctx, "search", json.RawMessage(`{"q":"hi"}`))
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Equal(t, "result for search", res.Text)

	res, err = c.CallTool(ctx, "fail", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Equal(t, "boom", res.Text)
}

func TestCallToolUnreachable(t *testing.T) {
	c := NewClient("http://127.0.0.1:1/unreachable")
	_, err := c.CallTool(context.Background(), "x", json.RawMessage(`{}`))
	assert.Error(t, err)
}

func TestListToolsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := NewClient(srv.URL)
	_, err := c.ListTools(context.Background())
	assert.Error(t, err)
}

var _ = strings.Contains // keep strings import if unused in future edits
