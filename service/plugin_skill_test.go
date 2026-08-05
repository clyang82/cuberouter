package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGithubSkillURL(t *testing.T) {
	cases := []struct {
		source  string
		want    string
		wantErr bool
	}{
		{"https://github.com/acme/web-search", "https://raw.githubusercontent.com/acme/web-search/HEAD/SKILL.md", false},
		{"https://github.com/acme/web-search/", "https://raw.githubusercontent.com/acme/web-search/HEAD/SKILL.md", false},
		{"https://github.com/acme/web-search/tree/main/skills/search", "https://raw.githubusercontent.com/acme/web-search/main/skills/search/SKILL.md", false},
		{"https://raw.githubusercontent.com/acme/web-search/main/SKILL.md", "https://raw.githubusercontent.com/acme/web-search/main/SKILL.md", false},
		{"https://example.com/not-github", "", true},
		{"not a url", "", true},
	}
	for _, tc := range cases {
		got, err := ResolveGithubSkillURL(tc.source)
		if tc.wantErr {
			assert.Error(t, err, tc.source)
			continue
		}
		require.NoError(t, err, tc.source)
		assert.Equal(t, tc.want, got)
	}
}

func TestFetchPluginSkill(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/ok.md"):
			fmt.Fprint(w, "# Skill\nDo the thing.")
		case strings.HasSuffix(r.URL.Path, "/empty.md"):
			// 200 with empty body
		case strings.HasSuffix(r.URL.Path, "/big.md"):
			w.Write(make([]byte, 300*1024))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	content, fetchedAt, err := FetchPluginSkill(srv.URL + "/ok.md")
	require.NoError(t, err)
	assert.Contains(t, content, "Do the thing.")
	assert.NotZero(t, fetchedAt)

	_, _, err = FetchPluginSkill(srv.URL + "/missing.md")
	assert.Error(t, err)

	_, _, err = FetchPluginSkill(srv.URL + "/empty.md")
	assert.Error(t, err)

	_, _, err = FetchPluginSkill(srv.URL + "/big.md")
	assert.Error(t, err) // exceeds 256 KiB cap
}
