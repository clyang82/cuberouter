package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ResendCampaignRewardEmail 重发活动兑换码邮件。
// admin 在活动详情页对某条已发放奖励触发；调用 service.SendCampaignRewardEmail
// 同步完成（让前端能拿到准确成败结果）。
func ResendCampaignRewardEmail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的奖励 ID")
		return
	}
	reward, err := model.GetCampaignRewardById(id)
	if err != nil {
		common.ApiErrorMsg(c, "奖励记录不存在")
		return
	}
	if reward.Status != model.CampaignRewardStatusDispatched || reward.RedemptionId == 0 {
		common.ApiErrorMsg(c, "该奖励未成功发放兑换码，无法补发邮件")
		return
	}
	campaign, err := model.GetCampaignById(reward.CampaignId)
	if err != nil {
		common.ApiErrorMsg(c, "活动不存在")
		return
	}
	redemption, err := model.GetRedemptionById(reward.RedemptionId)
	if err != nil {
		common.ApiErrorMsg(c, "兑换码不存在")
		return
	}
	user, err := model.GetUserById(reward.UserId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	if user.Email == "" {
		common.ApiErrorMsg(c, "目标用户未绑定邮箱")
		return
	}
	if err := service.SendCampaignRewardEmail(user, campaign, redemption, reward); err != nil {
		common.ApiErrorMsg(c, "邮件发送失败："+err.Error())
		return
	}
	common.ApiSuccess(c, nil)
}

// ============================================================
// Campaign CRUD Controllers (Admin)
// ============================================================

// GetAllCampaigns returns all campaigns with pagination
func GetAllCampaigns(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	total, campaigns, err := model.GetAllCampaigns(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(campaigns)
	common.ApiSuccess(c, pageInfo)
}

// SearchCampaigns searches campaigns by keyword
func SearchCampaigns(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	total, campaigns, err := model.SearchCampaigns(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(campaigns)
	common.ApiSuccess(c, pageInfo)
}

// GetCampaign returns a single campaign by ID
func GetCampaign(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的活动 ID")
		return
	}
	campaign, err := model.GetCampaignById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    campaign,
	})
}

// AddCampaign creates a new campaign
func AddCampaign(c *gin.Context) {
	campaign := &model.Campaign{}
	if err := common.DecodeJson(c.Request.Body, campaign); err != nil {
		common.ApiErrorMsg(c, "无效的请求参数")
		return
	}

	// Validate required fields
	if campaign.Name == "" {
		common.ApiErrorMsg(c, "活动名称不能为空")
		return
	}
	if campaign.Type == "" {
		common.ApiErrorMsg(c, "活动类型不能为空")
		return
	}

	// Validate campaign type
	validTypes := map[string]bool{
		model.CampaignTypePhoneFilled: true,
		model.CampaignTypeInvitation:  true,
	}
	if !validTypes[campaign.Type] {
		common.ApiErrorMsg(c, "无效的活动类型")
		return
	}

	// For invitation type, validate invitee_user_id and code_count
	if campaign.Type == model.CampaignTypeInvitation {
		config, err := campaign.ParseCampaignConfig()
		if err != nil {
			common.ApiErrorMsg(c, "活动配置解析失败")
			return
		}
		if config.InviteeUserId == 0 {
			common.ApiErrorMsg(c, "邀请活动必须指定关联用户")
			return
		}
		inviteeUser, err := model.GetUserById(config.InviteeUserId, false)
		if err != nil || inviteeUser == nil {
			common.ApiErrorMsg(c, "关联用户不存在")
			return
		}
		if config.CodeCount <= 0 {
			config.CodeCount = 1
		}
		if config.Quota <= 0 {
			common.ApiErrorMsg(c, "奖励额度必须大于0")
			return
		}
		// Force redemption_count to 1 — each dispatch always sends exactly 1 code
		config.RedemptionCount = 1
		// Write back the corrected config
		configJson, _ := common.Marshal(config)
		campaign.ConfigJson = string(configJson)
	}

	// Set defaults
	if campaign.Status == 0 {
		campaign.Status = model.CampaignStatusDraft
	}
	campaign.CreatedBy = c.GetInt("id")

	if err := campaign.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	// For invitation type, generate redemption codes (incremental)
	codesWarning := ""
	if campaign.Type == model.CampaignTypeInvitation && campaign.Status != model.CampaignStatusDraft {
		if _, err := service.CampaignEngineInstance.GenerateInvitationCodes(campaign); err != nil {
			common.SysError("GenerateInvitationCodes failed: " + err.Error())
			codesWarning = "兑换码生成失败: " + err.Error()
		}
	}

	// Audit log
	model.RecordLog(c.GetInt("id"), model.LogTypeManage,
		"创建活动: "+campaign.Name+" (ID: "+strconv.Itoa(campaign.Id)+")")

	message := "创建成功"
	if codesWarning != "" {
		message += "，但" + codesWarning
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    campaign,
		"warning": codesWarning,
	})
}

// updateCampaignRequest mirrors the Campaign JSON fields with pointers so that
// omitted fields (nil) can be distinguished from explicit zero values; a
// partial request must not clear stored values.
type updateCampaignRequest struct {
	Id          int     `json:"id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Type        *string `json:"type"`
	Status      *int    `json:"status"`
	StartAt     *int64  `json:"start_at"`
	EndAt       *int64  `json:"end_at"`
	ConfigJson  *string `json:"config_json"`
}

// UpdateCampaign updates an existing campaign
func UpdateCampaign(c *gin.Context) {
	var req updateCampaignRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的请求参数")
		return
	}

	if req.Id == 0 {
		common.ApiErrorMsg(c, "活动 ID 不能为空")
		return
	}

	// Verify campaign exists; the stored record is the merge target so omitted
	// request fields keep their current values.
	campaign, err := model.GetCampaignById(req.Id)
	if err != nil {
		common.ApiErrorMsg(c, "活动不存在")
		return
	}
	previousStatus := campaign.Status

	// Campaign type is immutable once created.
	if req.Type != nil && *req.Type != "" && *req.Type != campaign.Type {
		common.ApiErrorMsg(c, "活动类型创建后不可修改")
		return
	}

	// Merge only the supplied fields.
	if req.Name != nil {
		campaign.Name = *req.Name
	}
	if req.Description != nil {
		campaign.Description = *req.Description
	}
	if req.Status != nil {
		campaign.Status = *req.Status
	}
	if req.StartAt != nil {
		campaign.StartAt = *req.StartAt
	}
	if req.EndAt != nil {
		campaign.EndAt = *req.EndAt
	}
	if req.ConfigJson != nil {
		campaign.ConfigJson = *req.ConfigJson
	}

	if campaign.Name == "" {
		common.ApiErrorMsg(c, "活动名称不能为空")
		return
	}
	validStatuses := map[int]bool{
		model.CampaignStatusDraft:  true,
		model.CampaignStatusActive: true,
		model.CampaignStatusPaused: true,
		model.CampaignStatusEnded:  true,
	}
	if !validStatuses[campaign.Status] {
		common.ApiErrorMsg(c, "无效的活动状态")
		return
	}

	// For invitation type, validate invitation-specific fields
	if campaign.Type == model.CampaignTypeInvitation {
		config, err := campaign.ParseCampaignConfig()
		if err != nil {
			common.ApiErrorMsg(c, "活动配置解析失败")
			return
		}
		if config.InviteeUserId == 0 {
			common.ApiErrorMsg(c, "邀请活动必须指定关联用户")
			return
		}
		inviteeUser, err := model.GetUserById(config.InviteeUserId, false)
		if err != nil || inviteeUser == nil {
			common.ApiErrorMsg(c, "关联用户不存在")
			return
		}
		if config.CodeCount <= 0 {
			config.CodeCount = 1
		}
		if config.Quota <= 0 {
			common.ApiErrorMsg(c, "奖励额度必须大于0")
			return
		}
		// Force redemption_count to 1 — each dispatch always sends exactly 1 code
		config.RedemptionCount = 1
		// Write back the corrected config
		configJson, _ := common.Marshal(config)
		campaign.ConfigJson = string(configJson)
	}

	if err := campaign.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	// For invitation type being updated to active, generate redemption codes (incremental)
	codesWarning := ""
	if campaign.Type == model.CampaignTypeInvitation && (campaign.Status == model.CampaignStatusActive || previousStatus == model.CampaignStatusActive) {
		if generated, err := service.CampaignEngineInstance.GenerateInvitationCodes(campaign); err != nil {
			common.SysError("GenerateInvitationCodes on update failed: " + err.Error())
			codesWarning = "兑换码生成失败: " + err.Error()
		} else if generated > 0 {
			common.SysLog(fmt.Sprintf("GenerateInvitationCodes on update: generated %d codes for campaign %d", generated, campaign.Id))
		}
	}

	// Audit log
	model.RecordLog(c.GetInt("id"), model.LogTypeManage,
		"更新活动: "+campaign.Name+" (ID: "+strconv.Itoa(campaign.Id)+")")

	message := "更新成功"
	if codesWarning != "" {
		message += "，但" + codesWarning
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"warning": codesWarning,
	})
}

// UpdateCampaignStatus updates only the status of a campaign
func UpdateCampaignStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的活动 ID")
		return
	}

	var req struct {
		Status int `json:"status"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的请求参数")
		return
	}

	// Validate status
	validStatuses := map[int]bool{
		model.CampaignStatusDraft:  true,
		model.CampaignStatusActive: true,
		model.CampaignStatusPaused: true,
		model.CampaignStatusEnded:  true,
	}
	if !validStatuses[req.Status] {
		common.ApiErrorMsg(c, "无效的活动状态")
		return
	}

	campaign, err := model.GetCampaignById(id)
	if err != nil {
		common.ApiErrorMsg(c, "活动不存在")
		return
	}

	campaign.Status = req.Status
	if err := campaign.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	// For invitation type being activated, generate redemption codes (incremental)
	codesWarning := ""
	if campaign.Type == model.CampaignTypeInvitation && req.Status == model.CampaignStatusActive {
		if _, err := service.CampaignEngineInstance.GenerateInvitationCodes(campaign); err != nil {
			common.SysError("GenerateInvitationCodes on activate failed: " + err.Error())
			codesWarning = "兑换码生成失败: " + err.Error()
		}
	}

	// Audit log
	statusNames := map[int]string{
		model.CampaignStatusDraft:  "草稿",
		model.CampaignStatusActive: "进行中",
		model.CampaignStatusPaused: "已暂停",
		model.CampaignStatusEnded:  "已结束",
	}
	statusName := statusNames[req.Status]
	model.RecordLog(c.GetInt("id"), model.LogTypeManage,
		"更新活动状态: "+campaign.Name+" → "+statusName)

	message := "状态更新成功"
	if codesWarning != "" {
		message += "，但" + codesWarning
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"warning": codesWarning,
	})
}

// DeleteCampaign deletes a campaign
func DeleteCampaign(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的活动 ID")
		return
	}

	campaign, err := model.GetCampaignById(id)
	if err != nil {
		common.ApiErrorMsg(c, "活动不存在")
		return
	}

	if err := model.DeleteCampaignById(id); err != nil {
		common.ApiError(c, err)
		return
	}

	// Audit log
	model.RecordLog(c.GetInt("id"), model.LogTypeManage,
		"删除活动: "+campaign.Name+" (ID: "+strconv.Itoa(id)+")")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// ============================================================
// Campaign Statistics & Detail Controllers
// ============================================================

// GetCampaignStats returns statistics for a campaign
func GetCampaignStats(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的活动 ID")
		return
	}

	stats, err := model.GetCampaignStats(id)
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

// GetCampaignParticipants returns participants for a campaign
func GetCampaignParticipants(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的活动 ID")
		return
	}

	pageInfo := common.GetPageQuery(c)
	total, participants, err := model.GetCampaignParticipants(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Enrich with user info
	type ParticipantWithUser struct {
		model.CampaignParticipant
		Username string `json:"username"`
	}
	result := make([]ParticipantWithUser, 0, len(participants)) // empty page must marshal as [], not null
	for _, p := range participants {
		item := ParticipantWithUser{
			CampaignParticipant: *p,
		}
		user, err := model.GetUserById(p.UserId, false)
		if err == nil && user != nil {
			item.Username = user.Username
		}
		result = append(result, item)
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(result)
	common.ApiSuccess(c, pageInfo)
}

// GetCampaignRewards returns rewards for a campaign
func GetCampaignRewards(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的活动 ID")
		return
	}

	pageInfo := common.GetPageQuery(c)
	total, rewards, err := model.GetCampaignRewards(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Enrich with user info
	type RewardWithUser struct {
		model.CampaignReward
		Username string `json:"username"`
	}
	result := make([]RewardWithUser, 0, len(rewards)) // empty page must marshal as [], not null
	for _, r := range rewards {
		item := RewardWithUser{
			CampaignReward: *r,
		}
		user, err := model.GetUserById(r.UserId, false)
		if err == nil && user != nil {
			item.Username = user.Username
		}
		result = append(result, item)
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(result)
	common.ApiSuccess(c, pageInfo)
}
