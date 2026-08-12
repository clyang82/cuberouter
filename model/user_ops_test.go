package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertOpsUser(t *testing.T, username string, inviterId int, overrides ...func(*User)) *User {
	t.Helper()
	u := &User{
		Username: username, Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", InviterId: inviterId,
		AffCode: "aff-" + username,
	}
	for _, apply := range overrides {
		apply(u)
	}
	require.NoError(t, DB.Create(u).Error)
	return u
}

func TestGetOpsInviteesScopesAndOrders(t *testing.T) {
	truncateTables(t)
	// inviter A: two invitees (newer first); inviter B: one invitee (must never leak)
	a := insertOpsUser(t, "ops-a", 0)
	b := insertOpsUser(t, "ops-b", 0)
	old := insertOpsUser(t, "invitee-old", a.Id, func(u *User) { u.CreatedAt = 1000 })
	new := insertOpsUser(t, "invitee-new", a.Id, func(u *User) {
		u.CreatedAt = 2000
		u.SetAccessToken("invitee-token")
	})
	insertOpsUser(t, "invitee-of-b", b.Id)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	users, total, err := GetOpsInvitees(a.Id, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, users, 2)
	// ordered created_at DESC, id DESC
	assert.Equal(t, new.Id, users[0].Id)
	assert.Equal(t, old.Id, users[1].Id)
	for _, u := range users {
		assert.Equal(t, "", u.Password, "password must be omitted")
		assert.Nil(t, u.AccessToken, "access token must be omitted")
	}
}

func TestGetOpsInviteesPagination(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "ops-paged", 0)
	created := make([]*User, 0, 5)
	for i := 0; i < 5; i++ {
		// explicit created_at so the DESC ordering is deterministic
		created = append(created, insertOpsUser(t, "paged-user-"+string(rune('a'+i)), inviter.Id,
			func(u *User) { u.CreatedAt = int64(1000 + i) }))
	}
	pageInfo := &common.PageInfo{Page: 2, PageSize: 2}
	users, total, err := GetOpsInvitees(inviter.Id, pageInfo)
	require.NoError(t, err)
	assert.EqualValues(t, 5, total)
	// newest first: page 1 = [created[4], created[3]], page 2 = [created[2], created[1]]
	require.Len(t, users, 2)
	assert.Equal(t, created[2].Id, users[0].Id)
	assert.Equal(t, created[1].Id, users[1].Id)
}

func TestSearchOpsInviteesKeywordBehavior(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "ops-search", 0)
	target := insertOpsUser(t, "search-target", inviter.Id, func(u *User) {
		u.Email = "target@example.com"
		u.Phone = "13800001111"
	})
	insertOpsUser(t, "search-other", inviter.Id)

	// numeric keyword: id exact OR the four LIKE fields
	users, total, err := SearchOpsInvitees(inviter.Id, "search-target", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, target.Id, users[0].Id)

	users, total, err = SearchOpsInvitees(inviter.Id, "13800001111", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, target.Id, users[0].Id)

	// no match
	_, total, err = SearchOpsInvitees(inviter.Id, "nonexistent", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, total)
}

func TestSearchOpsInviteesScopedToInviter(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "ops-scope", 0)
	other := insertOpsUser(t, "ops-scope-other", 0)
	insertOpsUser(t, "scope-own", inviter.Id)
	insertOpsUser(t, "scope-foreign", other.Id)

	users, total, err := SearchOpsInvitees(inviter.Id, "", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, users, 1)
	assert.Equal(t, "scope-own", users[0].Username)
}

func TestExportOpsInviteesByIdsFiltersForeignAndBatches(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "ops-export", 0)
	other := insertOpsUser(t, "ops-export-other", 0)
	ids := make([]int, 0, opsExportBatchSize+1)
	for i := 0; i < opsExportBatchSize+1; i++ {
		u := insertOpsUser(t, "export-batch-"+string(rune('a'+i)), inviter.Id)
		ids = append(ids, u.Id)
	}
	foreign := insertOpsUser(t, "export-foreign", other.Id)
	ids = append(ids, foreign.Id)

	users, err := ExportOpsInviteesByIds(inviter.Id, ids)
	require.NoError(t, err)
	assert.Len(t, users, opsExportBatchSize+1)
	for _, u := range users {
		assert.Equal(t, inviter.Id, u.InviterId)
		assert.Equal(t, "", u.Password)
	}
}

func TestExportOpsInviteesByKeywordPaginates(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "ops-export-keyword", 0)
	insertOpsUser(t, "ops-export-other", 0)
	names := make([]string, 0, 205)
	for i := 0; i < 205; i++ {
		names = append(names, "kw-match-"+string(rune('a'+i%26))+string(rune('0'+i/26%10)))
	}
	for _, name := range names {
		insertOpsUser(t, name, inviter.Id)
	}
	users, err := ExportOpsInviteesByKeyword(inviter.Id, "kw-match-")
	require.NoError(t, err)
	assert.Len(t, users, 205)
	for _, u := range users {
		assert.Equal(t, inviter.Id, u.InviterId)
	}
}

func TestExportOpsInviteesByIdsRejectsTooManyIds(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "ops-export-cap-ids", 0)
	// The ids do not need to exist: the size guard runs before any query.
	ids := make([]int, opsExportMaxIds+1)
	_, err := ExportOpsInviteesByIds(inviter.Id, ids)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many ids")
}

func TestExportOpsInviteesByKeywordRejectsTooManyRows(t *testing.T) {
	truncateTables(t)
	inviter := insertOpsUser(t, "ops-export-cap-rows", 0)
	users := make([]User, 0, opsExportMaxRows+1)
	for i := 0; i < opsExportMaxRows+1; i++ {
		users = append(users, User{
			Username:  "kw-cap-" + strconv.Itoa(i),
			Password:  "password-placeholder",
			Role:      common.RoleCommonUser,
			Status:    common.UserStatusEnabled,
			Group:     "default",
			InviterId: inviter.Id,
			AffCode:   "aff-" + strconv.Itoa(i),
		})
	}
	// Small batches so the multi-row INSERT stays under SQLite's SQL variable limit.
	require.NoError(t, DB.CreateInBatches(&users, 10).Error)

	_, err := ExportOpsInviteesByKeyword(inviter.Id, "kw-cap-")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many invitees")
}
