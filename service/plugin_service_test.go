package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDBForPluginService(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.Plugin{}))
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM plugins")
		InvalidatePluginCache()
	})
}

func TestEnabledPluginCache(t *testing.T) {
	setupTestDBForPluginService(t)

	require.NoError(t, (&model.Plugin{Name: "A", Slug: "aaa", Enabled: true, McpUrl: "http://x"}).Insert())
	require.NoError(t, (&model.Plugin{Name: "B", Slug: "bbb", Enabled: false, McpUrl: "http://y"}).Insert())

	InvalidatePluginCache()
	got := GetEnabledPlugins()
	require.Len(t, got, 1)
	assert.Equal(t, "aaa", got[0].Slug)
}

func TestListPluginTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "initialize" || req.Method == "notifications/initialized" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0", "id": req.ID,
			"result": map[string]any{"tools": []map[string]any{{"name": "t1", "description": "d", "inputSchema": map[string]any{"type": "object"}}}},
		})
	}))
	defer srv.Close()

	tools, err := ListPluginTools(context.Background(), &model.Plugin{McpUrl: srv.URL})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "t1", tools[0].Name)
}

func TestRefreshPluginSkillFailureKeepsContent(t *testing.T) {
	setupTestDBForPluginService(t)
	p := &model.Plugin{Name: "A", Slug: "aaa", Enabled: true, McpUrl: "http://x", SkillSource: "http://127.0.0.1:1/nope"}
	require.NoError(t, p.Insert())

	warn := RefreshPluginSkill(p)
	assert.NotEmpty(t, warn)

	reloaded, err := model.GetPluginByID(p.Id)
	require.NoError(t, err)
	assert.Empty(t, reloaded.SkillContent)
}
