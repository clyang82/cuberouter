package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOpsCampaignTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Campaign{}, &model.CampaignParticipant{}, &model.CampaignReward{}))

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func insertOpsCampaign(t *testing.T, db *gorm.DB, name string, inviteeId int) *model.Campaign {
	t.Helper()
	config := model.CampaignConfig{InviteeUserId: inviteeId, InviteeUsername: "invitee"}
	configJson, err := common.Marshal(config)
	require.NoError(t, err)
	campaign := &model.Campaign{
		Name: name, Type: model.CampaignTypeInvitation, Status: model.CampaignStatusActive,
		StartAt: common.GetTimestamp() - 100, EndAt: 0, ConfigJson: string(configJson),
	}
	require.NoError(t, db.Create(campaign).Error)
	return campaign
}

func newOpsCampaignContext(t *testing.T, method, url string, userId int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, url, nil)
	c.Set("id", userId)
	return c, recorder
}

func TestGetOpsCampaignScopedToInvitee(t *testing.T) {
	db := setupOpsCampaignTestDB(t)
	opsA := model.User{Username: "ops-campaign-a", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-campaign-a-aff"}
	require.NoError(t, db.Create(&opsA).Error)
	campaign := insertOpsCampaign(t, db, "ops-a-campaign", opsA.Id)
	otherCampaign := insertOpsCampaign(t, db, "ops-b-campaign", 999999)

	// matching campaign: success
	c, recorder := newOpsCampaignContext(t, http.MethodGet, "/api/ops/campaign/"+fmt.Sprintf("%d", campaign.Id), opsA.Id)
	c.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", campaign.Id)}}
	GetOpsCampaign(c)
	var body struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)

	// foreign campaign: rejected
	c, recorder = newOpsCampaignContext(t, http.MethodGet, "/api/ops/campaign/"+fmt.Sprintf("%d", otherCampaign.Id), opsA.Id)
	c.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", otherCampaign.Id)}}
	GetOpsCampaign(c)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.False(t, body.Success)
}

func TestGetOpsCampaignStatsParticipantsRewardsScoped(t *testing.T) {
	db := setupOpsCampaignTestDB(t)
	opsA := model.User{Username: "ops-campaign-stats", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-campaign-stats-aff"}
	require.NoError(t, db.Create(&opsA).Error)
	mine := insertOpsCampaign(t, db, "mine-stats", opsA.Id)
	foreign := insertOpsCampaign(t, db, "foreign-stats", 999999)

	for _, tc := range []struct {
		name   string
		msgKey string
		route  func(c *gin.Context, id int)
	}{
		{name: "stats", msgKey: i18n.MsgOpsCampaignStatsNotAccessible, route: func(c *gin.Context, id int) {
			c.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", id)}}
			GetOpsCampaignStats(c)
		}},
		{name: "participants", msgKey: i18n.MsgOpsCampaignParticipantsNotAccessible, route: func(c *gin.Context, id int) {
			c.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", id)}}
			GetOpsCampaignParticipants(c)
		}},
		{name: "rewards", msgKey: i18n.MsgOpsCampaignRewardsNotAccessible, route: func(c *gin.Context, id int) {
			c.Params = []gin.Param{{Key: "id", Value: fmt.Sprintf("%d", id)}}
			GetOpsCampaignRewards(c)
		}},
	} {
		t.Run(tc.name+"-foreign-rejected", func(t *testing.T) {
			c, recorder := newOpsCampaignContext(t, http.MethodGet, "/api/ops/campaign/1/stats", opsA.Id)
			tc.route(c, foreign.Id)
			var body struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			assert.False(t, body.Success)
			assert.Equal(t, tc.msgKey, body.Message)
		})
		t.Run(tc.name+"-own-allowed", func(t *testing.T) {
			c, recorder := newOpsCampaignContext(t, http.MethodGet, "/api/ops/campaign/1/stats", opsA.Id)
			tc.route(c, mine.Id)
			var body struct {
				Success bool `json:"success"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
			assert.True(t, body.Success)
		})
	}
}

func TestGetOpsCampaignsAndSearchScoped(t *testing.T) {
	db := setupOpsCampaignTestDB(t)
	opsA := model.User{Username: "ops-campaign-list", Password: "password", Role: common.RoleOpsUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "ops-campaign-list-aff"}
	require.NoError(t, db.Create(&opsA).Error)
	insertOpsCampaign(t, db, "list-mine", opsA.Id)
	insertOpsCampaign(t, db, "list-mine-77", 77) // LIKE false positive for userId 7 family
	insertOpsCampaign(t, db, "list-foreign", 999999)

	c, recorder := newOpsCampaignContext(t, http.MethodGet, "/api/ops/campaign/?p=1&page_size=10", opsA.Id)
	GetOpsCampaigns(c)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Items []model.Campaign `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	assert.Equal(t, "list-mine", body.Data.Items[0].Name)

	c, recorder = newOpsCampaignContext(t, http.MethodGet, "/api/ops/campaign/search?keyword=list-mine&p=1&page_size=10", opsA.Id)
	SearchOpsCampaigns(c)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.Equal(t, 1, body.Data.Total)
}
