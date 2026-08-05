package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePluginMcpURL(t *testing.T) {
	assert.True(t, validatePluginMcpURL("http://localhost:8080/mcp"))
	assert.True(t, validatePluginMcpURL("https://mcp.example.com/sse"))
	assert.False(t, validatePluginMcpURL("ftp://example.com"))
	assert.False(t, validatePluginMcpURL("example.com/mcp"))
	assert.False(t, validatePluginMcpURL(""))
}

func newJSONTestContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/plugin/test", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// TestTestPluginConnectionRejectsNonHTTPScheme: the /test endpoint must
// reject MCP URLs without an http(s) scheme before dialing anything.
func TestTestPluginConnectionRejectsNonHTTPScheme(t *testing.T) {
	c, w := newJSONTestContext(`{"mcp_url":"ftp://example.com/mcp"}`)
	TestPluginConnection(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "http://")
}

// TestCreatePluginRejectsNonHTTPScheme: create must reject bad schemes too.
func TestCreatePluginRejectsNonHTTPScheme(t *testing.T) {
	c, w := newJSONTestContext(`{"name":"n","slug":"ab","mcp_url":"ws://example.com/mcp"}`)
	CreatePlugin(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "http://")
}

// TestUpdatePluginRejectsNonHTTPScheme: update must reject bad schemes too.
func TestUpdatePluginRejectsNonHTTPScheme(t *testing.T) {
	c, w := newJSONTestContext(`{"id":1,"name":"n","mcp_url":"example.com/mcp"}`)
	UpdatePlugin(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "http://")
}

