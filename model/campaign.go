package model

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// ============================================================
// Campaign Status Constants
// ============================================================

const (
	CampaignStatusDraft  = 1 // 草稿
	CampaignStatusActive = 2 // 进行中
	CampaignStatusPaused = 3 // 已暂停
	CampaignStatusEnded  = 4 // 已结束
)

// Campaign Type Constants
const (
	CampaignTypePhoneFilled = "phone_filled" // 填写手机号奖励
	CampaignTypeInvitation  = "invitation"   // 邀请活动
)

// Campaign Reward Status
const (
	CampaignRewardStatusPending    = 1 // 待发放
	CampaignRewardStatusDispatched = 2 // 已发放
	CampaignRewardStatusFailed     = 3 // 发放失败
	CampaignRewardStatusCancelled  = 4 // 已取消
)

// ============================================================
// Campaign Model
// ============================================================

// Campaign represents a marketing campaign/activity
type Campaign struct {
	Id          int            `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"size:255;not null"`
	Description string         `json:"description" gorm:"size:1024"`
	Type        string         `json:"type" gorm:"size:64;not null;index"`
	Status      int            `json:"status" gorm:"default:1;index"`
	StartAt     int64          `json:"start_at" gorm:"bigint"`
	EndAt       int64          `json:"end_at" gorm:"bigint"`
	ConfigJson  string         `json:"config_json" gorm:"type:text"`
	CreatedBy   int            `json:"created_by" gorm:"index"`
	CreatedAt   int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// CampaignConfig stores campaign-specific configuration as JSON
type CampaignConfig struct {
	Quota             int    `json:"quota"`                // 奖励额度
	RedemptionName    string `json:"redemption_name"`      // 兑换码名称前缀
	RedemptionCount   int    `json:"redemption_count"`     // 每次发放兑换码数量
	MaxParticipants   int    `json:"max_participants"`     // 最大参与人数，0 表示不限制
	MaxRewardsPerUser int    `json:"max_rewards_per_user"` // 每用户最大奖励次数，0 表示不限制
	ExpireDays        int    `json:"expire_days"`          // 兑换码 N 天后过期；0 表示不过期

	// invitation 类型专用字段
	InviteeUserId   int    `json:"invitee_user_id"`  // 被邀请用户（兑换码前缀关联用户）ID
	InviteeUsername string `json:"invitee_username"` // 被邀请用户用户名（用于前端回显）
	CodeCount       int    `json:"code_count"`       // 批量生成兑换码数量
}

// TableName specifies the table name for Campaign
func (Campaign) TableName() string {
	return "campaigns"
}

// ============================================================
// CampaignParticipant Model
// ============================================================

// CampaignParticipant records each participation/trigger event
type CampaignParticipant struct {
	Id         int    `json:"id" gorm:"primaryKey"`
	CampaignId int    `json:"campaign_id" gorm:"index;not null"`
	UserId     int    `json:"user_id" gorm:"index;not null"`
	EventType  string `json:"event_type" gorm:"size:64;not null;index"`
	EventAt    int64  `json:"event_at" gorm:"bigint"`
	ExtraJson  string `json:"extra_json" gorm:"type:text"`
}

// ParticipantExtra stores extra info for a participation event
type ParticipantExtra struct {
	InviterId   int    `json:"inviter_id,omitempty"`
	InviterName string `json:"inviter_name,omitempty"`
	Note        string `json:"note,omitempty"`
}

// TableName specifies the table name for CampaignParticipant
func (CampaignParticipant) TableName() string {
	return "campaign_participants"
}

// ============================================================
// CampaignReward Model
// ============================================================

// CampaignReward records rewards dispatched for a campaign
type CampaignReward struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	CampaignId   int    `json:"campaign_id" gorm:"index;not null"`
	UserId       int    `json:"user_id" gorm:"index;not null"`
	RedemptionId int    `json:"redemption_id" gorm:"index"`
	Quota        int    `json:"quota" gorm:"default:0"`
	Status       int    `json:"status" gorm:"default:1;index"`
	DispatchedAt int64  `json:"dispatched_at" gorm:"bigint"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	EmailSentAt  int64  `json:"email_sent_at" gorm:"bigint;default:0"` // 兑换码邮件最后一次成功发送时间；0=未发送
	EmailError   string `json:"email_error" gorm:"type:varchar(255)"`  // 最近一次发送失败的错误信息；成功后清空
}

// TableName specifies the table name for CampaignReward
func (CampaignReward) TableName() string {
	return "campaign_rewards"
}

// ============================================================
// Campaign CRUD
// ============================================================

func GetAllCampaigns(startIdx int, num int) (total int64, campaigns []*Campaign, err error) {
	err = DB.Model(&Campaign{}).Count(&total).Error
	if err != nil {
		return 0, nil, err
	}
	campaigns = make([]*Campaign, 0) // empty page must marshal as [], not null
	err = DB.Order("id desc").Limit(num).Offset(startIdx).Find(&campaigns).Error
	return total, campaigns, err
}

func SearchCampaigns(keyword string, startIdx int, num int) (total int64, campaigns []*Campaign, err error) {
	query := DB.Model(&Campaign{})
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	err = query.Count(&total).Error
	if err != nil {
		return 0, nil, err
	}
	campaigns = make([]*Campaign, 0) // empty page must marshal as [], not null
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&campaigns).Error
	return total, campaigns, err
}

func GetCampaignById(id int) (*Campaign, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	campaign := &Campaign{}
	err := DB.First(campaign, "id = ?", id).Error
	return campaign, err
}

// GetActiveCampaignsByType returns all active campaigns matching the given type
func GetActiveCampaignsByType(campaignType string) ([]*Campaign, error) {
	campaigns := make([]*Campaign, 0) // empty result must marshal as [], not null
	now := common.GetTimestamp()
	err := DB.Where("type = ? AND status = ? AND start_at <= ? AND (end_at = 0 OR end_at > ?)",
		campaignType, CampaignStatusActive, now, now).Find(&campaigns).Error
	return campaigns, err
}

func (campaign *Campaign) Insert() error {
	campaign.CreatedAt = common.GetTimestamp()
	campaign.UpdatedAt = common.GetTimestamp()
	return DB.Create(campaign).Error
}

func (campaign *Campaign) Update() error {
	campaign.UpdatedAt = common.GetTimestamp()
	return DB.Model(campaign).Select("name", "description", "type", "status",
		"start_at", "end_at", "config_json", "updated_at").Updates(campaign).Error
}

func (campaign *Campaign) Delete() error {
	return DB.Delete(campaign).Error
}

func DeleteCampaignById(id int) error {
	if id == 0 {
		return errors.New("id 为空")
	}
	campaign := Campaign{Id: id}
	if err := DB.First(&campaign, "id = ?", id).Error; err != nil {
		return err
	}
	return campaign.Delete()
}

// ============================================================
// CampaignParticipant CRUD
// ============================================================

func CreateCampaignParticipant(participant *CampaignParticipant) error {
	participant.EventAt = common.GetTimestamp()
	return DB.Create(participant).Error
}

// Admission sentinel errors: the campaign is full or the user has reached the
// per-user reward limit. Callers treat these as a silent skip, not a failure.
var (
	ErrCampaignFull            = errors.New("campaign participant limit reached")
	ErrCampaignUserRewardLimit = errors.New("campaign per-user reward limit reached")
)

// CreateCampaignParticipantIfAllowed atomically enforces MaxParticipants and
// MaxRewardsPerUser while inserting the participant row. Locking the campaign
// row serializes admission per campaign, so concurrent triggers cannot both
// pass the count checks (the previous count-then-insert sequence could
// over-admit under concurrency).
func CreateCampaignParticipantIfAllowed(participant *CampaignParticipant, maxParticipants int, maxRewardsPerUser int) error {
	participant.EventAt = common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		campaign := &Campaign{}
		if err := lockForUpdate(tx).First(campaign, "id = ?", participant.CampaignId).Error; err != nil {
			return err
		}
		if maxParticipants > 0 {
			var count int64
			if err := tx.Model(&CampaignParticipant{}).
				Where("campaign_id = ?", participant.CampaignId).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(maxParticipants) {
				return ErrCampaignFull
			}
		}
		if maxRewardsPerUser > 0 {
			var count int64
			if err := tx.Model(&CampaignParticipant{}).
				Where("campaign_id = ? AND user_id = ?", participant.CampaignId, participant.UserId).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(maxRewardsPerUser) {
				return ErrCampaignUserRewardLimit
			}
		}
		return tx.Create(participant).Error
	})
}

// DeleteCampaignParticipantById removes a participant row, e.g. to release the
// admission slot when reward dispatch found no available code.
func DeleteCampaignParticipantById(id int) error {
	if id == 0 {
		return errors.New("id 为空")
	}
	return DB.Delete(&CampaignParticipant{}, id).Error
}

// CountCampaignParticipants returns the number of participants for a campaign
func CountCampaignParticipants(campaignId int) (int64, error) {
	var count int64
	err := DB.Model(&CampaignParticipant{}).Where("campaign_id = ?", campaignId).Count(&count).Error
	return count, err
}

// CountCampaignParticipantsByUser returns the number of times a user has participated in a campaign
func CountCampaignParticipantsByUser(campaignId int, userId int) (int64, error) {
	var count int64
	err := DB.Model(&CampaignParticipant{}).Where("campaign_id = ? AND user_id = ?", campaignId, userId).Count(&count).Error
	return count, err
}

// GetCampaignParticipants returns paginated participants for a campaign
func GetCampaignParticipants(campaignId int, startIdx int, num int) (total int64, participants []*CampaignParticipant, err error) {
	query := DB.Model(&CampaignParticipant{}).Where("campaign_id = ?", campaignId)
	err = query.Count(&total).Error
	if err != nil {
		return 0, nil, err
	}
	participants = make([]*CampaignParticipant, 0) // empty page must marshal as [], not null
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&participants).Error
	return total, participants, err
}

// ============================================================
// CampaignReward CRUD
// ============================================================

func CreateCampaignReward(reward *CampaignReward) error {
	reward.CreatedAt = common.GetTimestamp()
	return DB.Create(reward).Error
}

// CountCampaignRewards returns the number of rewards for a campaign
func CountCampaignRewards(campaignId int) (int64, error) {
	var count int64
	err := DB.Model(&CampaignReward{}).Where("campaign_id = ?", campaignId).Count(&count).Error
	return count, err
}

// CountDispatchedRewards returns the number of dispatched rewards for a campaign
func CountDispatchedRewards(campaignId int) (int64, error) {
	var count int64
	err := DB.Model(&CampaignReward{}).Where("campaign_id = ? AND status = ?", campaignId, CampaignRewardStatusDispatched).Count(&count).Error
	return count, err
}

// SumDispatchedQuota returns the total quota dispatched for a campaign
func SumDispatchedQuota(campaignId int) (int64, error) {
	var total int64
	err := DB.Model(&CampaignReward{}).Where("campaign_id = ? AND status = ?", campaignId, CampaignRewardStatusDispatched).
		Select("COALESCE(SUM(quota), 0)").Scan(&total).Error
	return total, err
}

// GetCampaignRewards returns paginated rewards for a campaign
func GetCampaignRewards(campaignId int, startIdx int, num int) (total int64, rewards []*CampaignReward, err error) {
	query := DB.Model(&CampaignReward{}).Where("campaign_id = ?", campaignId)
	err = query.Count(&total).Error
	if err != nil {
		return 0, nil, err
	}
	rewards = make([]*CampaignReward, 0) // empty page must marshal as [], not null
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&rewards).Error
	return total, rewards, err
}

// HasPendingReward checks if a user has a pending reward for a campaign
func HasPendingReward(campaignId int, userId int) (bool, error) {
	var count int64
	err := DB.Model(&CampaignReward{}).Where("campaign_id = ? AND user_id = ? AND status = ?",
		campaignId, userId, CampaignRewardStatusPending).Count(&count).Error
	return count > 0, err
}

// GetCampaignRewardById 按 ID 加载奖励记录，找不到时返回 gorm.ErrRecordNotFound。
func GetCampaignRewardById(id int) (*CampaignReward, error) {
	var reward CampaignReward
	if err := DB.First(&reward, id).Error; err != nil {
		return nil, err
	}
	return &reward, nil
}

// MarkRewardEmailSent 标记奖励邮件发送成功（写入时间戳并清空旧的错误信息）。
// 使用 map[string]any 避免 GORM struct Updates 跳过零值字段 — EmailError 需要显式清空。
func MarkRewardEmailSent(rewardId int, at int64) error {
	return DB.Model(&CampaignReward{}).Where("id = ?", rewardId).
		Updates(map[string]any{"email_sent_at": at, "email_error": ""}).Error
}

// MarkRewardEmailFailed 记录邮件发送失败原因；不改动 EmailSentAt（保留上次成功时间）。
// 截断 errMsg 到 200 字符避免超长错误信息溢出 varchar(255)。
func MarkRewardEmailFailed(rewardId int, errMsg string) error {
	if len(errMsg) > 200 {
		errMsg = errMsg[:200]
	}
	return DB.Model(&CampaignReward{}).Where("id = ?", rewardId).
		Update("email_error", errMsg).Error
}

// ============================================================
// Campaign Statistics
// ============================================================

// CampaignStats holds statistics for a campaign
type CampaignStats struct {
	ParticipantCount int64 `json:"participant_count"`
	RewardCount      int64 `json:"reward_count"`
	DispatchedCount  int64 `json:"dispatched_count"`
	TotalQuota       int64 `json:"total_quota"`
}

// GetCampaignStats returns statistics for a campaign
func GetCampaignStats(campaignId int) (*CampaignStats, error) {
	stats := &CampaignStats{}
	var err error

	stats.ParticipantCount, err = CountCampaignParticipants(campaignId)
	if err != nil {
		return nil, err
	}

	stats.RewardCount, err = CountCampaignRewards(campaignId)
	if err != nil {
		return nil, err
	}

	stats.DispatchedCount, err = CountDispatchedRewards(campaignId)
	if err != nil {
		return nil, err
	}

	stats.TotalQuota, err = SumDispatchedQuota(campaignId)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// RecordCampaignLog records a log entry related to campaign activity
func RecordCampaignLog(userId int, logType int, content string) {
	RecordLog(userId, logType, content)
}

// RecordCampaignLogf records a formatted log entry
func RecordCampaignLogf(userId int, logType int, format string, args ...interface{}) {
	RecordLog(userId, logType, fmt.Sprintf(format, args...))
}

// ============================================================
// Helper: Parse campaign config
// ============================================================

// ParseCampaignConfig parses the ConfigJson field into CampaignConfig struct
func (c *Campaign) ParseCampaignConfig() (*CampaignConfig, error) {
	if c.ConfigJson == "" {
		return &CampaignConfig{}, nil
	}
	config := &CampaignConfig{}
	err := common.Unmarshal([]byte(c.ConfigJson), config)
	if err != nil {
		return nil, fmt.Errorf("解析活动配置失败: %w", err)
	}
	return config, nil
}

// Logger helper for campaign
func campaignLog(format string, args ...interface{}) {
	common.SysLog(fmt.Sprintf("[Campaign] "+format, args...))
}

// ============================================================
// Ops (运营角色) Campaign Queries — invitee_user_id == caller
// ============================================================

// GetCampaignsByInviteeUserId returns campaigns whose config_json contains
// invitee_user_id equal to the given user id. The DB side uses a LIKE
// pre-filter; the Go side parses config_json for a precise match so LIKE
// false positives (e.g. :7 matching :77) are excluded.
func GetCampaignsByInviteeUserId(userId int, startIdx int, num int) (campaigns []*Campaign, total int64, err error) {
	likePattern := fmt.Sprintf("%%\"invitee_user_id\":%d%%", userId)
	var allCampaigns []*Campaign
	if err := DB.Where("config_json LIKE ?", likePattern).Order("id desc").Find(&allCampaigns).Error; err != nil {
		return nil, 0, err
	}

	var filtered []*Campaign
	for _, c := range allCampaigns {
		config, parseErr := c.ParseCampaignConfig()
		if parseErr != nil {
			continue
		}
		if config.InviteeUserId == userId {
			filtered = append(filtered, c)
		}
	}

	total = int64(len(filtered))
	if startIdx >= len(filtered) {
		return []*Campaign{}, total, nil
	}
	end := startIdx + num
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[startIdx:end], total, nil
}

// SearchCampaignsByInviteeUserId searches the user's invitee-scoped campaigns
// by keyword (name prefix, or id exact for numeric keywords).
func SearchCampaignsByInviteeUserId(userId int, keyword string, startIdx int, num int) (campaigns []*Campaign, total int64, err error) {
	likePattern := fmt.Sprintf("%%\"invitee_user_id\":%d%%", userId)
	var allCampaigns []*Campaign
	query := DB.Where("config_json LIKE ?", likePattern)
	if id, parseErr := strconv.Atoi(keyword); parseErr == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	if err := query.Order("id desc").Find(&allCampaigns).Error; err != nil {
		return nil, 0, err
	}

	var filtered []*Campaign
	for _, c := range allCampaigns {
		config, parseErr := c.ParseCampaignConfig()
		if parseErr != nil {
			continue
		}
		if config.InviteeUserId == userId {
			filtered = append(filtered, c)
		}
	}

	total = int64(len(filtered))
	if startIdx >= len(filtered) {
		return []*Campaign{}, total, nil
	}
	end := startIdx + num
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[startIdx:end], total, nil
}
