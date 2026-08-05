// Package mcp is a minimal JSON-RPC 2.0 client for remote MCP servers
// (streamable HTTP transport). Supports optional Bearer-token auth.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	connectTimeout   = 5 * time.Second
	listToolsTimeout = 1800 * time.Second // temporary: raised from 10s for slow MCP servers
	callToolTimeout  = 1800 * time.Second // temporary: raised from 30s for slow MCP servers
	maxResponseBytes = 1 << 20            // 1 MiB
	toolListCacheTTL = 60 * time.Second
	protocolVersion  = "2025-03-26"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type CallResult struct {
	Text    string
	IsError bool
}

type Client struct {
	endpoint   string
	authHeader string
	authToken  string
	httpClient *http.Client

	mu          sync.Mutex
	toolsCache  []Tool
	toolsCached time.Time
}

func NewClient(endpoint string) *Client {
	return NewClientWithAuth(endpoint, "", "")
}

// NewClientWithAuth builds a client that authenticates every request when
// authToken is non-empty. With authHeader empty it sends
// `Authorization: Bearer <authToken>`; otherwise it sends
// `<authHeader>: <authToken>` (e.g. X-Auth-Token).
func NewClientWithAuth(endpoint, authHeader, authToken string) *Client {
	return &Client{
		endpoint:   strings.TrimRight(endpoint, "/"),
		authHeader: authHeader,
		authToken:  authToken,
		httpClient: &http.Client{
			Timeout: callToolTimeout + connectTimeout,
		},
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	ID     any             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, sessionID *string, req rpcRequest) (*rpcResponse, error) {
	payload, err := common.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if c.authToken != "" {
		if c.authHeader != "" {
			httpReq.Header.Set(c.authHeader, c.authToken)
		} else {
			httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
		}
	}
	if sessionID != nil && *sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", *sessionID)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if sid := resp.Header.Get("Mcp-Session-Id"); sessionID != nil && sid != "" {
		*sessionID = sid
	}
	if resp.StatusCode == http.StatusAccepted && req.ID == 0 {
		return nil, nil // notification acknowledged
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("mcp: response exceeds %d bytes", maxResponseBytes)
	}
	// Streamable HTTP servers may answer with SSE frames; take the last
	// `data:` payload, which carries the JSON-RPC response.
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/event-stream") {
		body = lastSSEData(body)
	}
	var rpcResp rpcResponse
	if err := common.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("mcp: decode response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp: rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return &rpcResp, nil
}

func lastSSEData(body []byte) []byte {
	var last []byte
	for _, line := range strings.Split(string(body), "\n") {
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			last = []byte(strings.TrimSpace(data))
		}
	}
	if last == nil {
		return body
	}
	return last
}

// initialize performs the MCP handshake on a fresh logical session.
func (c *Client) initialize(ctx context.Context) (string, error) {
	sessionID := ""
	_, err := c.do(ctx, &sessionID, rpcRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "new-api", "version": "1.0"},
		},
	})
	if err != nil {
		return "", err
	}
	// Best-effort initialized notification; servers that reject it still work.
	_, _ = c.do(ctx, &sessionID, rpcRequest{
		JSONRPC: "2.0", Method: "notifications/initialized",
	})
	return sessionID, nil
}

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	c.mu.Lock()
	if time.Since(c.toolsCached) < toolListCacheTTL && c.toolsCache != nil {
		tools := c.toolsCache
		c.mu.Unlock()
		return tools, nil
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, listToolsTimeout)
	defer cancel()
	sessionID, err := c.initialize(ctx)
	if err != nil {
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}
	resp, err := c.do(ctx, &sessionID, rpcRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := common.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp: decode tools/list: %w", err)
	}

	c.mu.Lock()
	c.toolsCache = result.Tools
	c.toolsCached = time.Now()
	c.mu.Unlock()
	return result.Tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (CallResult, error) {
	ctx, cancel := context.WithTimeout(ctx, callToolTimeout)
	defer cancel()
	sessionID, err := c.initialize(ctx)
	if err != nil {
		return CallResult{}, fmt.Errorf("mcp initialize: %w", err)
	}
	resp, err := c.do(ctx, &sessionID, rpcRequest{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": name, "arguments": arguments},
	})
	if err != nil {
		return CallResult{}, err
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := common.Unmarshal(resp.Result, &result); err != nil {
		return CallResult{}, fmt.Errorf("mcp: decode tools/call: %w", err)
	}
	var sb strings.Builder
	for _, part := range result.Content {
		if part.Type == "text" {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(part.Text)
		}
	}
	return CallResult{Text: sb.String(), IsError: result.IsError}, nil
}
