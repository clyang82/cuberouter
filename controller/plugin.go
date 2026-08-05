package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func validatePluginMcpURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// maskPluginAuthToken clears the token for API responses so the raw secret
// never leaves the admin API. The frontend sends it back only when changing
// it; an empty value on update means "keep the stored token".
func maskPluginAuthToken(p *model.Plugin) {
	p.AuthToken = ""
}

func GetPlugins(c *gin.Context) {
	plugins, err := model.GetAllPlugins()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for _, p := range plugins {
		maskPluginAuthToken(p)
	}
	common.ApiSuccess(c, plugins)
}

func CreatePlugin(c *gin.Context) {
	var p model.Plugin
	if err := c.ShouldBindJSON(&p); err != nil {
		common.ApiError(c, err)
		return
	}
	if p.Name == "" || p.Slug == "" || p.McpUrl == "" {
		common.ApiErrorMsg(c, "name, slug and MCP URL are required")
		return
	}
	if !validatePluginMcpURL(p.McpUrl) {
		common.ApiErrorMsg(c, "MCP URL must start with http:// or https://")
		return
	}
	if !model.ValidatePluginSlug(p.Slug) {
		common.ApiErrorMsg(c, "invalid slug: lowercase letters, digits and dashes, 2-64 chars, must not start with a dash")
		return
	}
	if dup, err := model.IsPluginSlugDuplicated(0, p.Slug); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "slug already exists")
		return
	}
	if err := p.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	warn := ""
	if p.SkillSource != "" {
		warn = service.RefreshPluginSkill(&p)
	}
	service.InvalidatePluginCache()
	maskPluginAuthToken(&p)
	c.JSON(200, gin.H{"success": true, "message": warn, "data": &p})
}

func UpdatePlugin(c *gin.Context) {
	var p model.Plugin
	if err := c.ShouldBindJSON(&p); err != nil {
		common.ApiError(c, err)
		return
	}
	if p.Id == 0 {
		common.ApiErrorMsg(c, "missing plugin id")
		return
	}
	if p.Name == "" || p.McpUrl == "" {
		common.ApiErrorMsg(c, "name and MCP URL are required")
		return
	}
	if !validatePluginMcpURL(p.McpUrl) {
		common.ApiErrorMsg(c, "MCP URL must start with http:// or https://")
		return
	}
	existing, err := model.GetPluginByID(p.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// Slug is immutable after creation.
	p.Slug = existing.Slug
	// Empty auth_token on update means "keep the stored token"; the field is
	// masked in GET responses, so the client can only send a real value when
	// the admin explicitly types a new one.
	if p.AuthToken == "" {
		p.AuthToken = existing.AuthToken
	}
	// Server-owned fields are not part of the update payload; preserve them.
	// A full-object save would otherwise zero created_time on every edit and
	// wipe the cached skill even when the source did not change.
	p.CreatedTime = existing.CreatedTime
	if p.SkillSource != existing.SkillSource {
		p.SkillContent = ""
		p.SkillFetchedAt = 0
	} else {
		p.SkillContent = existing.SkillContent
		p.SkillFetchedAt = existing.SkillFetchedAt
	}
	if err := p.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	warn := ""
	if p.SkillSource != "" && p.SkillContent == "" {
		warn = service.RefreshPluginSkill(&p)
	}
	service.InvalidatePluginCache()
	maskPluginAuthToken(&p)
	c.JSON(200, gin.H{"success": true, "message": warn, "data": &p})
}

func DeletePlugin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeletePluginByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidatePluginCache()
	common.ApiSuccess(c, nil)
}

func RefreshPlugin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	p, err := model.GetPluginByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if p.SkillSource == "" {
		common.ApiErrorMsg(c, "plugin has no skill source")
		return
	}
	warn := service.RefreshPluginSkill(p)
	if warn != "" {
		common.ApiErrorMsg(c, warn)
		return
	}
	common.ApiSuccess(c, p)
}

func TestPluginConnection(c *gin.Context) {
	var req struct {
		McpUrl     string `json:"mcp_url"`
		AuthHeader string `json:"auth_header"`
		AuthToken  string `json:"auth_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.McpUrl == "" {
		common.ApiErrorMsg(c, "mcp_url is required")
		return
	}
	if !validatePluginMcpURL(req.McpUrl) {
		common.ApiErrorMsg(c, "MCP URL must start with http:// or https://")
		return
	}
	tools, err := service.ListPluginTools(c.Request.Context(), &model.Plugin{McpUrl: req.McpUrl, AuthHeader: req.AuthHeader, AuthToken: req.AuthToken})
	if err != nil {
		common.ApiErrorMsg(c, "connection failed: "+err.Error())
		return
	}
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	common.ApiSuccess(c, gin.H{"tools": names})
}

// GetEnabledPluginList serves the playground @ autocomplete.
// Exposes only slug/name/description — never MCP URLs or skill content.
func GetEnabledPluginList(c *gin.Context) {
	type item struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	plugins := service.GetEnabledPlugins()
	items := make([]item, 0, len(plugins))
	for _, p := range plugins {
		items = append(items, item{Slug: p.Slug, Name: p.Name, Description: p.Description})
	}
	common.ApiSuccess(c, items)
}
