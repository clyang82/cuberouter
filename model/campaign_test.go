package model

import (
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertCampaign(t *testing.T, c *Campaign) *Campaign {
	t.Helper()
	require.NoError(t, c.Insert())
	require.NotZero(t, c.Id)
	return c
}

func TestGetActiveCampaignsByType(t *testing.T) {
	now := common.GetTimestamp()
	fixtures := []*Campaign{
		{Name: "active-in-window", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive, StartAt: now - 100, EndAt: now + 100},
		{Name: "active-no-end", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive, StartAt: now - 100, EndAt: 0},
		{Name: "draft", Type: CampaignTypePhoneFilled, Status: CampaignStatusDraft, StartAt: now - 100, EndAt: now + 100},
		{Name: "paused", Type: CampaignTypePhoneFilled, Status: CampaignStatusPaused, StartAt: now - 100, EndAt: now + 100},
		{Name: "ended", Type: CampaignTypePhoneFilled, Status: CampaignStatusEnded, StartAt: now - 100, EndAt: now + 100},
		{Name: "not-started", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive, StartAt: now + 500, EndAt: now + 1000},
		{Name: "expired-window", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive, StartAt: now - 1000, EndAt: now - 500},
		{Name: "other-type", Type: CampaignTypeInvitation, Status: CampaignStatusActive, StartAt: now - 100, EndAt: now + 100},
	}
	for _, c := range fixtures {
		insertCampaign(t, c)
	}

	campaigns, err := GetActiveCampaignsByType(CampaignTypePhoneFilled)
	require.NoError(t, err)
	names := make([]string, 0, len(campaigns))
	for _, c := range campaigns {
		names = append(names, c.Name)
	}
	assert.ElementsMatch(t, []string{"active-in-window", "active-no-end"}, names)
}

func TestSearchCampaigns(t *testing.T) {
	insertCampaign(t, &Campaign{Name: "alpha promo", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})
	insertCampaign(t, &Campaign{Name: "alphabet soup", Type: CampaignTypePhoneFilled, Status: CampaignStatusDraft})
	beta := insertCampaign(t, &Campaign{Name: "beta", Type: CampaignTypeInvitation, Status: CampaignStatusActive})

	total, items, err := SearchCampaigns("alph", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 2)

	total, _, err = SearchCampaigns("gamma", 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)

	// Numeric keyword matches by exact id or name prefix.
	total, items, err = SearchCampaigns(strconv.Itoa(beta.Id), 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "beta", items[0].Name)

	// Pagination: page 2 of size 1 over the 2 "alph" matches.
	total, items, err = SearchCampaigns("alph", 1, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, items, 1)
}

func TestCampaignRewardCounts(t *testing.T) {
	c1 := insertCampaign(t, &Campaign{Name: "rw-1", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})
	c2 := insertCampaign(t, &Campaign{Name: "rw-2", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})

	rewards := []*CampaignReward{
		{CampaignId: c1.Id, UserId: 100, RedemptionId: 1, Quota: 100, Status: CampaignRewardStatusDispatched},
		{CampaignId: c1.Id, UserId: 101, RedemptionId: 2, Quota: 200, Status: CampaignRewardStatusDispatched},
		{CampaignId: c1.Id, UserId: 102, Quota: 50, Status: CampaignRewardStatusPending},
		{CampaignId: c1.Id, UserId: 103, Quota: 75, Status: CampaignRewardStatusFailed},
		{CampaignId: c2.Id, UserId: 100, RedemptionId: 3, Quota: 200, Status: CampaignRewardStatusDispatched},
	}
	for _, r := range rewards {
		require.NoError(t, CreateCampaignReward(r))
	}

	total, err := CountCampaignRewards(c1.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)

	dispatched, err := CountDispatchedRewards(c1.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(2), dispatched)

	sum, err := SumDispatchedQuota(c1.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(300), sum)

	sum, err = SumDispatchedQuota(999999)
	require.NoError(t, err)
	assert.Equal(t, int64(0), sum)

	pending, err := HasPendingReward(c1.Id, 102)
	require.NoError(t, err)
	assert.True(t, pending)

	pending, err = HasPendingReward(c1.Id, 100)
	require.NoError(t, err)
	assert.False(t, pending)
}

func TestCampaignParticipantCounts(t *testing.T) {
	c1 := insertCampaign(t, &Campaign{Name: "pt-1", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})
	c2 := insertCampaign(t, &Campaign{Name: "pt-2", Type: CampaignTypeInvitation, Status: CampaignStatusActive})

	require.NoError(t, CreateCampaignParticipant(&CampaignParticipant{CampaignId: c1.Id, UserId: 100, EventType: CampaignTypePhoneFilled}))
	require.NoError(t, CreateCampaignParticipant(&CampaignParticipant{CampaignId: c1.Id, UserId: 100, EventType: CampaignTypePhoneFilled}))
	require.NoError(t, CreateCampaignParticipant(&CampaignParticipant{CampaignId: c1.Id, UserId: 101, EventType: CampaignTypePhoneFilled}))
	require.NoError(t, CreateCampaignParticipant(&CampaignParticipant{CampaignId: c2.Id, UserId: 100, EventType: CampaignTypeInvitation}))

	total, err := CountCampaignParticipants(c1.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	byUser, err := CountCampaignParticipantsByUser(c1.Id, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(2), byUser)

	total, items, err := GetCampaignParticipants(c1.Id, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, items, 3)
	assert.NotZero(t, items[0].EventAt, "EventAt must be stamped on insert")

	total, items, err = GetCampaignParticipants(c1.Id, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, items, 1)
}

func TestMarkRewardEmailSentClearsError(t *testing.T) {
	c := insertCampaign(t, &Campaign{Name: "mail-1", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})
	r := &CampaignReward{CampaignId: c.Id, UserId: 100, Quota: 10, Status: CampaignRewardStatusDispatched, EmailError: "SMTP timeout"}
	require.NoError(t, CreateCampaignReward(r))

	require.NoError(t, MarkRewardEmailSent(r.Id, 12345))
	reloaded, err := GetCampaignRewardById(r.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), reloaded.EmailSentAt)
	assert.Equal(t, "", reloaded.EmailError, "map-based update must write the empty string")
}

func TestMarkRewardEmailFailed(t *testing.T) {
	c := insertCampaign(t, &Campaign{Name: "mail-2", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})
	r := &CampaignReward{CampaignId: c.Id, UserId: 100, Quota: 10, Status: CampaignRewardStatusDispatched, DispatchedAt: 999}
	require.NoError(t, CreateCampaignReward(r))

	longErr := strings.Repeat("x", 300)
	require.NoError(t, MarkRewardEmailFailed(r.Id, longErr))
	reloaded, err := GetCampaignRewardById(r.Id)
	require.NoError(t, err)
	assert.Len(t, reloaded.EmailError, 200, "error message truncated to 200 chars")
	assert.Equal(t, int64(999), reloaded.DispatchedAt)
}

func TestParseCampaignConfig(t *testing.T) {
	empty := &Campaign{}
	cfg, err := empty.ParseCampaignConfig()
	require.NoError(t, err)
	assert.Equal(t, CampaignConfig{}, *cfg)

	bad := &Campaign{ConfigJson: "{not-json"}
	_, err = bad.ParseCampaignConfig()
	assert.Error(t, err)

	full := &Campaign{ConfigJson: `{"quota":500,"redemption_name":"Summer","redemption_count":1,"max_participants":10,"max_rewards_per_user":2,"expire_days":30,"invitee_user_id":42,"invitee_username":"boss","code_count":100}`}
	cfg, err = full.ParseCampaignConfig()
	require.NoError(t, err)
	assert.Equal(t, 500, cfg.Quota)
	assert.Equal(t, "Summer", cfg.RedemptionName)
	assert.Equal(t, 1, cfg.RedemptionCount)
	assert.Equal(t, 10, cfg.MaxParticipants)
	assert.Equal(t, 2, cfg.MaxRewardsPerUser)
	assert.Equal(t, 30, cfg.ExpireDays)
	assert.Equal(t, 42, cfg.InviteeUserId)
	assert.Equal(t, "boss", cfg.InviteeUsername)
	assert.Equal(t, 100, cfg.CodeCount)
}

func TestGetCampaignStats(t *testing.T) {
	c := insertCampaign(t, &Campaign{Name: "stats-1", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})
	require.NoError(t, CreateCampaignParticipant(&CampaignParticipant{CampaignId: c.Id, UserId: 100, EventType: CampaignTypePhoneFilled}))
	require.NoError(t, CreateCampaignParticipant(&CampaignParticipant{CampaignId: c.Id, UserId: 101, EventType: CampaignTypePhoneFilled}))
	require.NoError(t, CreateCampaignReward(&CampaignReward{CampaignId: c.Id, UserId: 100, RedemptionId: 1, Quota: 100, Status: CampaignRewardStatusDispatched}))
	require.NoError(t, CreateCampaignReward(&CampaignReward{CampaignId: c.Id, UserId: 101, Quota: 250, Status: CampaignRewardStatusPending}))

	stats, err := GetCampaignStats(c.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.ParticipantCount)
	assert.Equal(t, int64(2), stats.RewardCount)
	assert.Equal(t, int64(1), stats.DispatchedCount)
	assert.Equal(t, int64(100), stats.TotalQuota)
}

// Empty lists must serialize as "items":[] — a nil slice marshals to null, which
// breaks strict zod parsing of the paged responses on the frontend.
// Kept last: it wipes campaign tables; other tests above already completed.
func TestCampaignListGettersReturnNonNilEmptySlices(t *testing.T) {
	require.NoError(t, DB.Exec("DELETE FROM campaign_rewards").Error)
	require.NoError(t, DB.Exec("DELETE FROM campaign_participants").Error)
	require.NoError(t, DB.Exec("DELETE FROM campaigns").Error)

	total, campaigns, err := GetAllCampaigns(0, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.NotNil(t, campaigns)
	assert.Len(t, campaigns, 0)

	total, campaigns, err = SearchCampaigns("no-such-campaign", 0, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.NotNil(t, campaigns)
	assert.Len(t, campaigns, 0)

	actives, err := GetActiveCampaignsByType(CampaignTypePhoneFilled)
	require.NoError(t, err)
	assert.NotNil(t, actives)
	assert.Len(t, actives, 0)

	c := insertCampaign(t, &Campaign{Name: "fresh", Type: CampaignTypePhoneFilled, Status: CampaignStatusActive})

	total, participants, err := GetCampaignParticipants(c.Id, 0, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.NotNil(t, participants)
	assert.Len(t, participants, 0)

	total, rewards, err := GetCampaignRewards(c.Id, 0, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.NotNil(t, rewards)
	assert.Len(t, rewards, 0)
}

func campaignWithInvitee(t *testing.T, name string, inviteeId int) *Campaign {
	t.Helper()
	config := CampaignConfig{InviteeUserId: inviteeId, InviteeUsername: "invitee"}
	configJson, err := common.Marshal(config)
	require.NoError(t, err)
	return &Campaign{
		Name: name, Type: CampaignTypeInvitation, Status: CampaignStatusActive,
		StartAt: common.GetTimestamp() - 100, EndAt: 0, ConfigJson: string(configJson),
	}
}

func TestGetCampaignsByInviteeUserId(t *testing.T) {
	truncateTables(t)
	// 12 and 123 are LIKE false positives for each other; 1234 for both.
	insertCampaign(t, campaignWithInvitee(t, "mine", 12))
	insertCampaign(t, campaignWithInvitee(t, "not-mine-123", 123))
	insertCampaign(t, campaignWithInvitee(t, "not-mine-1234", 1234))
	insertCampaign(t, campaignWithInvitee(t, "no-invitee", 0))

	campaigns, total, err := GetCampaignsByInviteeUserId(12, 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, campaigns, 1)
	assert.Equal(t, "mine", campaigns[0].Name)

	// pagination beyond the filtered set returns empty page with correct total
	page, total, err := GetCampaignsByInviteeUserId(12, 10, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Empty(t, page)
}

func TestSearchCampaignsByInviteeUserId(t *testing.T) {
	truncateTables(t)
	a := insertCampaign(t, campaignWithInvitee(t, "search-campaign-a", 7))
	insertCampaign(t, campaignWithInvitee(t, "search-campaign-b", 77))
	insertCampaign(t, campaignWithInvitee(t, "foreign-campaign", 8))

	// keyword matches name prefix and stays scoped
	campaigns, total, err := SearchCampaignsByInviteeUserId(7, "search-campaign", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, campaigns, 1)
	assert.Equal(t, "search-campaign-a", campaigns[0].Name)

	// numeric keyword matches id exact (still scoped)
	campaigns, total, err = SearchCampaignsByInviteeUserId(7, strconv.Itoa(a.Id), 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, "search-campaign-a", campaigns[0].Name)
}
