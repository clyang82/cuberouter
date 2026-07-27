package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePluginSlug(t *testing.T) {
	cases := []struct {
		slug  string
		valid bool
	}{
		{"web-search", true},
		{"a1", true},
		{"ab", true},
		{"a", false},                      // too short
		{"-abc", false},                   // leading dash
		{"Web", false},                    // uppercase
		{"web_search", false},             // underscore
		{"web search", false},             // space
		{string(make([]byte, 65)), false}, // too long (also invalid chars)
	}
	for _, tc := range cases {
		assert.Equal(t, tc.valid, ValidatePluginSlug(tc.slug), "slug=%q", tc.slug)
	}
}

func setupTestDBForPlugin(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Plugin{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM plugins")
	})
}

func TestPluginCRUD(t *testing.T) {
	setupTestDBForPlugin(t)

	p := &Plugin{
		Name:        "Web Search",
		Slug:        "web-search",
		Description: "search the web",
		Enabled:     true,
		McpUrl:      "https://mcp.example.com/mcp",
		SkillSource: "https://github.com/acme/web-search-skill",
	}
	require.NoError(t, p.Insert())
	require.NotZero(t, p.Id)
	require.NotZero(t, p.CreatedTime)

	got, err := GetPluginByID(p.Id)
	require.NoError(t, err)
	assert.Equal(t, "web-search", got.Slug)
	assert.True(t, got.Enabled)

	dup, err := IsPluginSlugDuplicated(0, "web-search")
	require.NoError(t, err)
	assert.True(t, dup)
	dup, err = IsPluginSlugDuplicated(p.Id, "web-search")
	require.NoError(t, err)
	assert.False(t, dup)

	got.Enabled = false
	require.NoError(t, got.Update())
	all, err := GetAllPlugins()
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.False(t, all[0].Enabled)

	require.NoError(t, DeletePluginByID(p.Id))
	all, err = GetAllPlugins()
	require.NoError(t, err)
	assert.Empty(t, all)
}
