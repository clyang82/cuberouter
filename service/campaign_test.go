package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetCampaignTables(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Exec("DELETE FROM campaign_rewards").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM campaign_participants").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM campaigns").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM redemptions").Error)
}

var campaignUserSeq = 900000

func seedCampaignUser(t *testing.T) *model.User {
	t.Helper()
	campaignUserSeq++
	id := campaignUserSeq
	username := fmt.Sprintf("campaign-user-%d", id)
	user := &model.User{
		Id:          id,
		Username:    username,
		Password:    "x",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     fmt.Sprintf("aff-%d", id), // uniqueIndex — must be unique per user
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func insertActiveCampaign(t *testing.T, name, campaignType, configJson string) *model.Campaign {
	t.Helper()
	campaign := &model.Campaign{
		Name:       name,
		Type:       campaignType,
		Status:     model.CampaignStatusActive,
		StartAt:    common.GetTimestamp() - 60,
		EndAt:      0,
		ConfigJson: configJson,
	}
	require.NoError(t, campaign.Insert())
	require.NotZero(t, campaign.Id)
	return campaign
}

func countAllRewards(t *testing.T) int64 {
	t.Helper()
	var n int64
	require.NoError(t, model.DB.Model(&model.CampaignReward{}).Count(&n).Error)
	return n
}

func TestCampaignEngineGuards(t *testing.T) {
	resetCampaignTables(t)
	engine := CampaignEngineInstance

	// Guards are synchronous: invalid inputs must not schedule any goroutine.
	engine.OnPhoneFilled(nil)
	engine.OnPhoneFilled(&model.User{}) // Id == 0
	engine.OnInvitationRegister(nil, 900111)
	engine.OnInvitationRegister(&model.User{}, 900111)
	engine.OnInvitationRegister(seedCampaignUser(t), 0)

	// If a goroutine had been scheduled it would have finished within this window.
	time.Sleep(100 * time.Millisecond)
	var participants int64
	require.NoError(t, model.DB.Model(&model.CampaignParticipant{}).Count(&participants).Error)
	assert.Zero(t, participants)
	assert.Zero(t, countAllRewards(t))
}

func TestOnPhoneFilledDispatchesAsynchronously(t *testing.T) {
	resetCampaignTables(t)
	user := seedCampaignUser(t)
	insertActiveCampaign(t, "phone-e2e", model.CampaignTypePhoneFilled, `{"quota":50}`)

	start := time.Now()
	CampaignEngineInstance.OnPhoneFilled(user)
	assert.Less(t, time.Since(start), 50*time.Millisecond, "trigger must not block the caller")

	require.Eventually(t, func() bool {
		return countAllRewards(t) == 1
	}, 3*time.Second, 20*time.Millisecond)

	var reward model.CampaignReward
	require.NoError(t, model.DB.First(&reward).Error)
	assert.Equal(t, model.CampaignRewardStatusDispatched, reward.Status)
	assert.Equal(t, 50, reward.Quota)
	assert.NotZero(t, reward.RedemptionId)

	var code model.Redemption
	require.NoError(t, model.DB.First(&code, reward.RedemptionId).Error)
	assert.Equal(t, user.Id, code.UserId)
	assert.Zero(t, code.ExpiredTime, "expire_days=0 means the code never expires")

	var participants int64
	require.NoError(t, model.DB.Model(&model.CampaignParticipant{}).Count(&participants).Error)
	assert.Equal(t, int64(1), participants)
}

func TestProcessCampaignForUserCaps(t *testing.T) {
	resetCampaignTables(t)
	engine := CampaignEngineInstance
	user := seedCampaignUser(t)
	other := seedCampaignUser(t)

	campaign := insertActiveCampaign(t, "capped", model.CampaignTypePhoneFilled,
		`{"quota":10,"max_participants":1,"max_rewards_per_user":1}`)
	require.NoError(t, model.CreateCampaignParticipant(&model.CampaignParticipant{
		CampaignId: campaign.Id, UserId: other.Id, EventType: model.CampaignTypePhoneFilled,
	}))

	// MaxParticipants reached → skip entirely.
	engine.processCampaignForUser(campaign, user, 0)
	var mine int64
	require.NoError(t, model.DB.Model(&model.CampaignParticipant{}).Where("user_id = ?", user.Id).Count(&mine).Error)
	assert.Zero(t, mine)

	// Raise the global cap: first run dispatches, second is blocked by MaxRewardsPerUser.
	campaign.ConfigJson = `{"quota":10,"max_participants":10,"max_rewards_per_user":1}`
	engine.processCampaignForUser(campaign, user, 0)
	engine.processCampaignForUser(campaign, user, 0)
	require.NoError(t, model.DB.Model(&model.CampaignParticipant{}).Where("user_id = ?", user.Id).Count(&mine).Error)
	assert.Equal(t, int64(1), mine)
	var rewards int64
	require.NoError(t, model.DB.Model(&model.CampaignReward{}).Where("campaign_id = ?", campaign.Id).Count(&rewards).Error)
	assert.Equal(t, int64(1), rewards)
}

func TestProcessCampaignForUserRecordsInviter(t *testing.T) {
	resetCampaignTables(t)
	engine := CampaignEngineInstance
	inviter := seedCampaignUser(t)
	user := seedCampaignUser(t)
	campaign := insertActiveCampaign(t, "inv-extra", model.CampaignTypeInvitation,
		fmt.Sprintf(`{"quota":10,"invitee_user_id":%d}`, inviter.Id))

	// Seed a pool code so dispatch succeeds and the participant row survives;
	// the empty-pool release path is covered in TestDispatchInvitationReward.
	require.NoError(t, (&model.Redemption{
		UserId: inviter.Id, Key: common.GetUUID(), Status: common.RedemptionCodeStatusEnabled,
		Name: "pool", Quota: 10, CreatedTime: common.GetTimestamp(),
		OwnerAdminId: inviter.Id, CampaignId: campaign.Id,
	}).Insert())

	engine.processCampaignForUser(campaign, user, inviter.Id)

	var p model.CampaignParticipant
	require.NoError(t, model.DB.Where("campaign_id = ? AND user_id = ?", campaign.Id, user.Id).First(&p).Error)
	assert.Equal(t, model.CampaignTypeInvitation, p.EventType)
	var extra model.ParticipantExtra
	require.NoError(t, common.Unmarshal([]byte(p.ExtraJson), &extra))
	assert.Equal(t, inviter.Id, extra.InviterId)
	assert.Equal(t, inviter.Username, extra.InviterName)
}

func TestDispatchPhoneFilledReward(t *testing.T) {
	resetCampaignTables(t)
	engine := CampaignEngineInstance
	user := seedCampaignUser(t)

	t.Run("zero quota is a no-op", func(t *testing.T) {
		campaign := insertActiveCampaign(t, "phone-zero", model.CampaignTypePhoneFilled, `{"quota":0}`)
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		engine.dispatchPhoneFilledReward(campaign, user, cfg)
		assert.Zero(t, countAllRewards(t))
		var codes int64
		require.NoError(t, model.DB.Model(&model.Redemption{}).Count(&codes).Error)
		assert.Zero(t, codes)
	})

	t.Run("expire_days sets the code expiry", func(t *testing.T) {
		campaign := insertActiveCampaign(t, "phone-exp", model.CampaignTypePhoneFilled, `{"quota":88,"expire_days":10}`)
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		before := common.GetTimestamp()
		engine.dispatchPhoneFilledReward(campaign, user, cfg)
		after := common.GetTimestamp()

		var reward model.CampaignReward
		require.NoError(t, model.DB.First(&reward).Error)
		assert.Equal(t, model.CampaignRewardStatusDispatched, reward.Status)
		assert.Equal(t, 88, reward.Quota)
		var code model.Redemption
		require.NoError(t, model.DB.First(&code, reward.RedemptionId).Error)
		assert.GreaterOrEqual(t, code.ExpiredTime, before+10*86400)
		assert.LessOrEqual(t, code.ExpiredTime, after+10*86400)
		assert.Zero(t, code.OwnerAdminId, "phone_filled codes must not enter the invitation pool")
	})

	t.Run("insert failure records a failed reward", func(t *testing.T) {
		campaign := insertActiveCampaign(t, "phone-fail", model.CampaignTypePhoneFilled, `{"quota":77}`)
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		require.NoError(t, model.DB.Migrator().DropTable(&model.Redemption{}))
		t.Cleanup(func() {
			require.NoError(t, model.DB.AutoMigrate(&model.Redemption{}))
		})

		engine.dispatchPhoneFilledReward(campaign, user, cfg)

		var reward model.CampaignReward
		require.NoError(t, model.DB.Order("id desc").First(&reward).Error)
		assert.Equal(t, model.CampaignRewardStatusFailed, reward.Status)
		assert.Equal(t, 77, reward.Quota)
		assert.Zero(t, reward.RedemptionId)
	})
}

func TestDispatchInvitationReward(t *testing.T) {
	resetCampaignTables(t)
	engine := CampaignEngineInstance
	inviter := seedCampaignUser(t)
	user := seedCampaignUser(t)

	t.Run("invitee mismatch is a no-op", func(t *testing.T) {
		campaign := insertActiveCampaign(t, "inv-mismatch", model.CampaignTypeInvitation, `{"quota":99,"invitee_user_id":1}`)
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		engine.dispatchInvitationReward(campaign, user, inviter.Id, cfg, 0)
		assert.Zero(t, countAllRewards(t))
	})

	t.Run("empty pool records no reward and releases the participant slot", func(t *testing.T) {
		campaign := insertActiveCampaign(t, "inv-empty", model.CampaignTypeInvitation,
			fmt.Sprintf(`{"quota":99,"invitee_user_id":%d}`, inviter.Id))
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		participant := &model.CampaignParticipant{
			CampaignId: campaign.Id,
			UserId:     user.Id,
			EventType:  campaign.Type,
		}
		require.NoError(t, model.CreateCampaignParticipant(participant))
		engine.dispatchInvitationReward(campaign, user, inviter.Id, cfg, participant.Id)
		assert.Zero(t, countAllRewards(t), "empty pool must not create a reward row")
		count, err := model.CountCampaignParticipants(campaign.Id)
		require.NoError(t, err)
		assert.Zero(t, count, "empty pool must release the admission slot so a later top-up can reach the user")
	})

	t.Run("success draws from the pool with the pool quota", func(t *testing.T) {
		campaign := insertActiveCampaign(t, "inv-ok", model.CampaignTypeInvitation,
			fmt.Sprintf(`{"quota":99,"invitee_user_id":%d}`, inviter.Id))
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		code := &model.Redemption{
			UserId:       inviter.Id,
			Key:          common.GetUUID(),
			Status:       common.RedemptionCodeStatusEnabled,
			Name:         "pool",
			Quota:        321, // pool quota wins over campaign config quota
			CreatedTime:  common.GetTimestamp(),
			OwnerAdminId: inviter.Id,
			CampaignId:   campaign.Id,
		}
		require.NoError(t, code.Insert())

		engine.dispatchInvitationReward(campaign, user, inviter.Id, cfg, 0)

		var reward model.CampaignReward
		require.NoError(t, model.DB.Order("id desc").First(&reward).Error)
		assert.Equal(t, model.CampaignRewardStatusDispatched, reward.Status)
		assert.Equal(t, 321, reward.Quota)
		assert.Equal(t, code.Id, reward.RedemptionId)
		var after model.User
		require.NoError(t, model.DB.First(&after, user.Id).Error)
		assert.Equal(t, 321, after.Quota)
	})

	t.Run("dispatch error records a failed reward", func(t *testing.T) {
		campaign := insertActiveCampaign(t, "inv-fail", model.CampaignTypeInvitation,
			fmt.Sprintf(`{"quota":55,"invitee_user_id":%d}`, inviter.Id))
		cfg, err := campaign.ParseCampaignConfig()
		require.NoError(t, err)
		require.NoError(t, model.DB.Migrator().DropTable(&model.Redemption{}))
		t.Cleanup(func() {
			require.NoError(t, model.DB.AutoMigrate(&model.Redemption{}))
		})

		engine.dispatchInvitationReward(campaign, user, inviter.Id, cfg, 0)

		var reward model.CampaignReward
		require.NoError(t, model.DB.Order("id desc").First(&reward).Error)
		assert.Equal(t, model.CampaignRewardStatusFailed, reward.Status)
		assert.Zero(t, reward.RedemptionId)
	})
}

func TestGenerateInvitationCodes(t *testing.T) {
	resetCampaignTables(t)
	engine := CampaignEngineInstance
	inviter := seedCampaignUser(t)
	campaign := &model.Campaign{Name: "gen", Type: model.CampaignTypeInvitation}

	campaign.ConfigJson = `{"quota":10,"code_count":2}`
	_, err := engine.GenerateInvitationCodes(campaign)
	assert.EqualError(t, err, "邀请活动缺少关联用户")

	campaign.ConfigJson = `{"quota":10,"invitee_user_id":99999999,"code_count":2}`
	_, err = engine.GenerateInvitationCodes(campaign)
	assert.EqualError(t, err, "关联用户不存在")

	// quota guard fires only when codes actually need to be generated.
	campaign.ConfigJson = fmt.Sprintf(`{"quota":0,"invitee_user_id":%d,"code_count":5}`, inviter.Id)
	_, err = engine.GenerateInvitationCodes(campaign)
	assert.EqualError(t, err, "奖励额度必须大于0")

	campaign.ConfigJson = fmt.Sprintf(`{"quota":25,"invitee_user_id":%d,"code_count":3}`, inviter.Id)
	n, err := engine.GenerateInvitationCodes(campaign)
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	var codes []model.Redemption
	require.NoError(t, model.DB.Where("owner_admin_id = ?", inviter.Id).Find(&codes).Error)
	require.Len(t, codes, 3)
	for _, code := range codes {
		assert.LessOrEqual(t, len(code.Key), 32, "key must fit the char(32) column (prefix<=24 + 8)")
		assert.Equal(t, 25, code.Quota)
		assert.Equal(t, inviter.Id, code.UserId)
		assert.Equal(t, common.RedemptionCodeStatusEnabled, code.Status)
	}

	// Incremental: pool already at target size → generate nothing.
	n, err = engine.GenerateInvitationCodes(campaign)
	require.NoError(t, err)
	assert.Zero(t, n)

	// Incremental deficit: consume one code → exactly one is regenerated.
	require.NoError(t, model.DB.Model(&model.Redemption{}).Where("id = ?", codes[0].Id).
		Update("status", common.RedemptionCodeStatusUsed).Error)
	n, err = engine.GenerateInvitationCodes(campaign)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestGenerateInvitationCodesCap(t *testing.T) {
	resetCampaignTables(t)
	engine := CampaignEngineInstance
	inviter := seedCampaignUser(t)
	campaign := &model.Campaign{Name: "gen-cap", Type: model.CampaignTypeInvitation,
		ConfigJson: fmt.Sprintf(`{"quota":1,"invitee_user_id":%d,"code_count":1001}`, inviter.Id)}

	// code_count > 1000 is clamped to 1000; asserts the cap boundary itself.
	n, err := engine.GenerateInvitationCodes(campaign)
	require.NoError(t, err)
	assert.Equal(t, 1000, n)
	available, err := model.CountAvailableRedemptionCodesByOwner(campaign.Id, inviter.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), available)
}

func TestSendCampaignRewardEmail(t *testing.T) {
	resetCampaignTables(t)
	oldServer, oldAccount := common.SMTPServer, common.SMTPAccount
	common.SMTPServer, common.SMTPAccount = "", "" // deterministic "SMTP 服务器未配置"
	t.Cleanup(func() { common.SMTPServer, common.SMTPAccount = oldServer, oldAccount })

	campaign := insertActiveCampaign(t, "mail-svc", model.CampaignTypePhoneFilled, `{"quota":10}`)
	user := seedCampaignUser(t)
	code := &model.Redemption{UserId: user.Id, Key: common.GetUUID(), Status: common.RedemptionCodeStatusEnabled,
		Name: "x", Quota: 10, CreatedTime: common.GetTimestamp()}
	require.NoError(t, code.Insert())
	reward := &model.CampaignReward{CampaignId: campaign.Id, UserId: user.Id, RedemptionId: code.Id, Quota: 10,
		Status: model.CampaignRewardStatusDispatched, DispatchedAt: common.GetTimestamp()}
	require.NoError(t, model.CreateCampaignReward(reward))

	// No email bound → nil, nothing written.
	require.NoError(t, SendCampaignRewardEmail(user, campaign, code, reward))
	reloaded, err := model.GetCampaignRewardById(reward.Id)
	require.NoError(t, err)
	assert.Zero(t, reloaded.EmailSentAt)
	assert.Empty(t, reloaded.EmailError)

	// Email bound but SMTP unconfigured → error returned, EmailError persisted, EmailSentAt untouched.
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("email", "u@example.com").Error)
	withEmail, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	err = SendCampaignRewardEmail(withEmail, campaign, code, reward)
	require.Error(t, err)
	reloaded, err = model.GetCampaignRewardById(reward.Id)
	require.NoError(t, err)
	assert.NotEmpty(t, reloaded.EmailError)
	assert.Zero(t, reloaded.EmailSentAt)
}

// 3 of 4 phone_filled trigger sites (UpdateUser/UpdateSelf/CreateUser) pass a slim
// user struct without Email. The engine must reload the authoritative record so the
// reward email is attempted, not silently skipped.
func TestOnPhoneFilledReloadsSlimUserForRewardEmail(t *testing.T) {
	resetCampaignTables(t)
	oldServer, oldAccount := common.SMTPServer, common.SMTPAccount
	common.SMTPServer, common.SMTPAccount = "", "" // SendEmail fails deterministically offline
	t.Cleanup(func() { common.SMTPServer, common.SMTPAccount = oldServer, oldAccount })

	user := seedCampaignUser(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("email", "u@example.com").Error)
	insertActiveCampaign(t, "phone-slim", model.CampaignTypePhoneFilled, `{"quota":10}`)

	slim := &model.User{Id: user.Id, Username: user.Username} // no Email
	CampaignEngineInstance.OnPhoneFilled(slim)

	require.Eventually(t, func() bool {
		var reward model.CampaignReward
		if err := model.DB.First(&reward).Error; err != nil {
			return false
		}
		return reward.EmailSentAt > 0 || reward.EmailError != ""
	}, 3*time.Second, 20*time.Millisecond, "reward row must show an email attempt, not a silent skip")

	var reward model.CampaignReward
	require.NoError(t, model.DB.First(&reward).Error)
	assert.NotEmpty(t, reward.EmailError, "SMTP unset in tests ⇒ the attempted send must record an error, not a silent skip")
	assert.Zero(t, reward.EmailSentAt)
}

// Invitation dispatch marks the code used and credits quota atomically, so the
// reward email must be a credit receipt — sending "enter this code" instructions
// for an already-redeemed code would be a dead end for the user.
func TestBuildCampaignRewardEmailContract(t *testing.T) {
	redemption := &model.Redemption{Key: "PREFIX-ab12cd34", Quota: 100, ExpiredTime: 1700000000}

	invitation := &model.Campaign{Name: "inv", Type: model.CampaignTypeInvitation}
	subject, content := buildCampaignRewardEmail(invitation, redemption)
	assert.Contains(t, subject, "Campaign Reward Credited")
	assert.Contains(t, content, "credited to your account automatically")
	assert.Contains(t, content, "已自动充值")
	assert.Contains(t, content, "PREFIX-ab12cd34", "receipt still references the code as a voucher")
	assert.NotContains(t, content, "Please enter the redemption code",
		"an auto-credited code cannot be redeemed again — no redemption instructions")

	phoneFilled := &model.Campaign{Name: "phone", Type: model.CampaignTypePhoneFilled}
	subject, content = buildCampaignRewardEmail(phoneFilled, redemption)
	assert.Contains(t, subject, "Campaign Redemption Code")
	assert.Contains(t, content, "Please enter the redemption code")
	assert.Contains(t, content, "PREFIX-ab12cd34")
}
