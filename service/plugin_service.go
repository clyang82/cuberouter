package service

import (
	"context"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp"
)

var enabledPluginsCache struct {
	sync.RWMutex
	plugins []*model.Plugin
	loaded  bool
}

// InvalidatePluginCache forces the next GetEnabledPlugins to reload from DB.
// Called by every plugin mutating controller handler.
func InvalidatePluginCache() {
	enabledPluginsCache.Lock()
	defer enabledPluginsCache.Unlock()
	enabledPluginsCache.loaded = false
	enabledPluginsCache.plugins = nil
}

// GetEnabledPlugins returns enabled plugins, cached in memory. DB is the
// source of truth; cache is invalidated on every admin mutation.
func GetEnabledPlugins() []*model.Plugin {
	enabledPluginsCache.RLock()
	if enabledPluginsCache.loaded {
		defer enabledPluginsCache.RUnlock()
		return enabledPluginsCache.plugins
	}
	enabledPluginsCache.RUnlock()

	enabledPluginsCache.Lock()
	defer enabledPluginsCache.Unlock()
	if enabledPluginsCache.loaded {
		return enabledPluginsCache.plugins
	}
	var plugins []*model.Plugin
	if err := model.DB.Where("enabled = ?", true).Find(&plugins).Error; err != nil {
		common.SysError("failed to load enabled plugins: " + err.Error())
		enabledPluginsCache.plugins = nil
	} else {
		enabledPluginsCache.plugins = plugins
	}
	enabledPluginsCache.loaded = true
	return enabledPluginsCache.plugins
}

// ListPluginTools fetches the tool list of a plugin's MCP server.
// The mcp client caches per endpoint internally (60s TTL).
func ListPluginTools(ctx context.Context, p *model.Plugin) ([]mcp.Tool, error) {
	client := mcp.NewClientWithAuth(p.McpUrl, p.AuthHeader, p.AuthToken)
	return client.ListTools(ctx)
}

// RefreshPluginSkill fetches the plugin's skill from its GitHub source and
// persists the snapshot. On failure the stored content is left untouched and
// a human-readable warning is returned.
func RefreshPluginSkill(p *model.Plugin) string {
	content, fetchedAt, err := FetchPluginSkill(p.SkillSource)
	if err != nil {
		return "skill fetch failed: " + err.Error()
	}
	p.SkillContent = content
	p.SkillFetchedAt = fetchedAt
	if err := p.Update(); err != nil {
		return "skill save failed: " + err.Error()
	}
	InvalidatePluginCache()
	return ""
}
