package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertOwnerCode(t *testing.T, key string, campaignId, ownerId, quota, status int, expiredTime int64) *Redemption {
	t.Helper()
	r := &Redemption{
		UserId:       ownerId,
		Key:          key,
		Status:       status,
		Name:         "pool-code",
		Quota:        quota,
		CreatedTime:  common.GetTimestamp(),
		ExpiredTime:  expiredTime,
		OwnerAdminId: ownerId,
		CampaignId:   campaignId,
	}
	require.NoError(t, r.Insert())
	require.NotZero(t, r.Id)
	return r
}

func TestCountAvailableRedemptionCodesByOwner(t *testing.T) {
	now := common.GetTimestamp()
	owner := 7001
	insertOwnerCode(t, "cnt-00000001", 601, owner, 100, common.RedemptionCodeStatusEnabled, 0)
	insertOwnerCode(t, "cnt-00000002", 601, owner, 100, common.RedemptionCodeStatusEnabled, now+3600)
	insertOwnerCode(t, "cnt-00000003", 601, owner, 100, common.RedemptionCodeStatusUsed, 0)
	insertOwnerCode(t, "cnt-00000004", 601, owner, 100, common.RedemptionCodeStatusEnabled, now-3600) // expired
	insertOwnerCode(t, "cnt-00000005", 601, 7999, 100, common.RedemptionCodeStatusEnabled, 0)         // other owner
	insertOwnerCode(t, "cnt-00000006", 602, owner, 100, common.RedemptionCodeStatusEnabled, 0)        // same owner, other campaign

	count, err := CountAvailableRedemptionCodesByOwner(601, owner)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "pool is scoped by campaign and owner")

	count, err = CountAvailableRedemptionCodesByOwner(602, owner)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = CountAvailableRedemptionCodesByOwner(601, 7654)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func seedDispatchUser(t *testing.T, id int) *User {
	t.Helper()
	username := fmt.Sprintf("dispatch-user-%d", id)
	user := &User{
		Id:          id,
		Username:    username,
		Password:    "x",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     fmt.Sprintf("aff-%d", id), // uniqueIndex — must be unique per user
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestDispatchRedemptionToUser_EmptyPool(t *testing.T) {
	redemption, err := DispatchRedemptionToUser(701, 7101, 8000)
	require.NoError(t, err)
	assert.Nil(t, redemption, "empty pool returns (nil, nil), not an error")
}

func TestDispatchRedemptionToUser_CampaignIsolation(t *testing.T) {
	owner := 7151
	user := seedDispatchUser(t, 8051)
	insertOwnerCode(t, "iso-00000001", 711, owner, 100, common.RedemptionCodeStatusEnabled, 0)

	// Another campaign sharing the same invitee must not draw campaign 711's codes.
	redemption, err := DispatchRedemptionToUser(712, owner, user.Id)
	require.NoError(t, err)
	assert.Nil(t, redemption, "campaign 712 must not see campaign 711's pool")

	var after User
	require.NoError(t, DB.First(&after, user.Id).Error)
	assert.Zero(t, after.Quota, "no quota credited when the campaign pool is empty")
}

func TestDispatchRedemptionToUser_MissingRecipientRollsBack(t *testing.T) {
	owner := 7161
	code := insertOwnerCode(t, "rb-000000001", 721, owner, 100, common.RedemptionCodeStatusEnabled, 0)

	redemption, err := DispatchRedemptionToUser(721, owner, 987654321)
	require.Error(t, err, "crediting a nonexistent user must fail the dispatch")
	assert.Nil(t, redemption)

	reloaded, err := GetRedemptionById(code.Id)
	require.NoError(t, err)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, reloaded.Status,
		"code must stay available when the transaction rolls back")
}

func TestDispatchRedemptionToUser_Success(t *testing.T) {
	owner := 7201
	user := seedDispatchUser(t, 8001)
	code := insertOwnerCode(t, "ok-000000001", 731, owner, 250, common.RedemptionCodeStatusEnabled, 0)

	redemption, err := DispatchRedemptionToUser(731, owner, user.Id)
	require.NoError(t, err)
	require.NotNil(t, redemption)
	assert.Equal(t, code.Id, redemption.Id)
	assert.Equal(t, 250, redemption.Quota)

	reloaded, err := GetRedemptionById(code.Id)
	require.NoError(t, err)
	assert.Equal(t, common.RedemptionCodeStatusUsed, reloaded.Status)
	assert.Equal(t, user.Id, reloaded.UsedUserId)
	assert.NotZero(t, reloaded.RedeemedTime)

	var after User
	require.NoError(t, DB.First(&after, user.Id).Error)
	assert.Equal(t, 250, after.Quota)

	// Second dispatch from the now-empty pool returns (nil, nil).
	redemption, err = DispatchRedemptionToUser(731, owner, user.Id)
	require.NoError(t, err)
	assert.Nil(t, redemption)
}

func TestDispatchRedemptionToUser_ConcurrentNoDoubleIssue(t *testing.T) {
	owner := 7301
	const codes = 5
	const attempts = 20
	for i := 0; i < codes; i++ {
		insertOwnerCode(t, fmt.Sprintf("cc-%08d", i), 741, owner, 100, common.RedemptionCodeStatusEnabled, 0)
	}
	for i := 0; i < attempts; i++ {
		seedDispatchUser(t, 8100+i)
	}

	// sqlite in-memory with SetMaxOpenConns(1) serializes transactions; the CAS
	// (RowsAffected==0) branch is what would catch a lost lock on MySQL/Postgres.
	outcomes := make([]*Redemption, attempts)
	dispatchErrors := make([]error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r, err := DispatchRedemptionToUser(741, owner, 8100+idx)
			outcomes[idx] = r
			dispatchErrors[idx] = err
		}(i)
	}
	wg.Wait()
	for _, dispatchErr := range dispatchErrors {
		require.NoError(t, dispatchErr)
	}

	issued := make(map[int]bool)
	for _, r := range outcomes {
		if r != nil {
			assert.False(t, issued[r.Id], "code %d issued twice", r.Id)
			issued[r.Id] = true
		}
	}
	assert.Len(t, issued, codes, "exactly %d unique codes issued", codes)

	creditedUsers := 0
	for i := 0; i < attempts; i++ {
		var u User
		require.NoError(t, DB.First(&u, 8100+i).Error)
		if u.Quota == 100 {
			creditedUsers++
		} else {
			assert.Equal(t, 0, u.Quota, "user %d quota must be 0 or 100", 8100+i)
		}
	}
	assert.Equal(t, codes, creditedUsers, "exactly %d users credited", codes)

	remaining, err := CountAvailableRedemptionCodesByOwner(741, owner)
	require.NoError(t, err)
	assert.Equal(t, int64(0), remaining)
}
