package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPluginRoutesRegisterWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	require.NotPanics(t, func() {
		SetApiRouter(engine)
	})

	var pluginPaths []string
	for _, r := range engine.Routes() {
		if len(r.Path) >= len("/api/plugin") && r.Path[:len("/api/plugin")] == "/api/plugin" {
			pluginPaths = append(pluginPaths, r.Method+" "+r.Path)
		}
	}
	require.ElementsMatch(t, []string{
		"GET /api/plugin/enabled",
		"GET /api/plugin/",
		"POST /api/plugin/",
		"PUT /api/plugin/",
		"DELETE /api/plugin/:id",
		"POST /api/plugin/:id/refresh",
		"POST /api/plugin/test",
	}, pluginPaths)
}
