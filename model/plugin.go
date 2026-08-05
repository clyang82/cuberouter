package model

import (
	"regexp"

	"github.com/QuantumNous/new-api/common"
)

var PluginSlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

type Plugin struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	Name           string `json:"name" gorm:"size:128;not null"`
	Slug           string `json:"slug" gorm:"size:64;not null;uniqueIndex"`
	Description    string `json:"description" gorm:"size:512"`
	Enabled        bool   `json:"enabled"`
	McpUrl         string `json:"mcp_url" gorm:"size:1024"`
	AuthHeader     string `json:"auth_header" gorm:"size:128"`
	AuthToken      string `json:"auth_token,omitempty" gorm:"size:1024"`
	SkillSource    string `json:"skill_source" gorm:"size:1024"`
	SkillContent   string `json:"skill_content" gorm:"type:text"`
	SkillFetchedAt int64  `json:"skill_fetched_at" gorm:"bigint"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime    int64  `json:"updated_time" gorm:"bigint"`
}

func ValidatePluginSlug(slug string) bool {
	return PluginSlugRegex.MatchString(slug)
}

func (p *Plugin) Insert() error {
	now := common.GetTimestamp()
	p.CreatedTime = now
	p.UpdatedTime = now
	return DB.Create(p).Error
}

func (p *Plugin) Update() error {
	p.UpdatedTime = common.GetTimestamp()
	return DB.Save(p).Error
}

func DeletePluginByID(id int) error {
	return DB.Delete(&Plugin{}, id).Error
}

func GetAllPlugins() ([]*Plugin, error) {
	var plugins []*Plugin
	if err := DB.Model(&Plugin{}).Order("updated_time DESC").Find(&plugins).Error; err != nil {
		return nil, err
	}
	return plugins, nil
}

func GetPluginByID(id int) (*Plugin, error) {
	var p Plugin
	if err := DB.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func IsPluginSlugDuplicated(id int, slug string) (bool, error) {
	if slug == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Plugin{}).Where("slug = ? AND id <> ?", slug, id).Count(&cnt).Error
	return cnt > 0, err
}
