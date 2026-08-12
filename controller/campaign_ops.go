package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
)

// ============================================================
// Ops (运营角色) Campaign Controllers — read-only, invitee-scoped
// ============================================================

// loadOpsCampaign loads the campaign and returns the parsed config after
// verifying the current user is the campaign's invitee. Returns (nil, nil)
// when the request should be aborted (response already written).
func loadOpsCampaign(c *gin.Context, forbiddenMsgKey string) (*model.Campaign, *model.CampaignConfig) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgOpsInvalidCampaignId)
		return nil, nil
	}

	campaign, err := model.GetCampaignById(id)
	if err != nil {
		common.ApiError(c, err)
		return nil, nil
	}

	userId := c.GetInt("id")
	config, parseErr := campaign.ParseCampaignConfig()
	if parseErr != nil || config.InviteeUserId != userId {
		common.ApiErrorI18n(c, forbiddenMsgKey)
		return nil, nil
	}
	return campaign, config
}

func GetOpsCampaigns(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	campaigns, total, err := model.GetCampaignsByInviteeUserId(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(campaigns)
	common.ApiSuccess(c, pageInfo)
}

func SearchOpsCampaigns(c *gin.Context) {
	keyword := c.Query("keyword")
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	campaigns, total, err := model.SearchCampaignsByInviteeUserId(userId, keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(campaigns)
	common.ApiSuccess(c, pageInfo)
}

func GetOpsCampaign(c *gin.Context) {
	campaign, _ := loadOpsCampaign(c, i18n.MsgOpsCampaignNotAccessible)
	if campaign == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    campaign,
	})
}

func GetOpsCampaignStats(c *gin.Context) {
	campaign, _ := loadOpsCampaign(c, i18n.MsgOpsCampaignStatsNotAccessible)
	if campaign == nil {
		return
	}
	stats, err := model.GetCampaignStats(campaign.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

func GetOpsCampaignParticipants(c *gin.Context) {
	campaign, _ := loadOpsCampaign(c, i18n.MsgOpsCampaignParticipantsNotAccessible)
	if campaign == nil {
		return
	}

	pageInfo := common.GetPageQuery(c)
	total, participants, err := model.GetCampaignParticipants(campaign.Id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	type ParticipantWithUser struct {
		model.CampaignParticipant
		Username string `json:"username"`
	}
	userIds := make([]int, 0, len(participants))
	for _, p := range participants {
		userIds = append(userIds, p.UserId)
	}
	usersById, err := model.GetUsersByIds(userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]ParticipantWithUser, 0, len(participants))
	for _, p := range participants {
		item := ParticipantWithUser{CampaignParticipant: *p}
		if user, ok := usersById[p.UserId]; ok {
			item.Username = user.Username
		}
		result = append(result, item)
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(result)
	common.ApiSuccess(c, pageInfo)
}

func GetOpsCampaignRewards(c *gin.Context) {
	campaign, _ := loadOpsCampaign(c, i18n.MsgOpsCampaignRewardsNotAccessible)
	if campaign == nil {
		return
	}

	pageInfo := common.GetPageQuery(c)
	total, rewards, err := model.GetCampaignRewards(campaign.Id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	type RewardWithUser struct {
		model.CampaignReward
		Username string `json:"username"`
	}
	userIds := make([]int, 0, len(rewards))
	for _, r := range rewards {
		userIds = append(userIds, r.UserId)
	}
	usersById, err := model.GetUsersByIds(userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]RewardWithUser, 0, len(rewards))
	for _, r := range rewards {
		item := RewardWithUser{CampaignReward: *r}
		if user, ok := usersById[r.UserId]; ok {
			item.Username = user.Username
		}
		result = append(result, item)
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(result)
	common.ApiSuccess(c, pageInfo)
}
