package model

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ExportUsersByIds returns exactly the requested ids, batched at 200, with
// password/access_token omitted.
func TestExportUsersByIds(t *testing.T) {
	truncateTables(t)
	ids := make([]int, 0, 250)
	for i := 0; i < 250; i++ {
		u := insertOpsUser(t, fmt.Sprintf("export-%03d", i), 0, func(u *User) {
			u.CreatedAt = int64(1700000000 + i)
			u.SetAccessToken(fmt.Sprintf("export-token-%d", i))
		})
		ids = append(ids, u.Id)
	}

	got, err := ExportUsersByIds(ids)
	require.NoError(t, err)
	require.Len(t, got, 250)
	for _, u := range got {
		assert.Equal(t, "", u.Password, "password must be omitted")
		assert.Nil(t, u.AccessToken, "access token must be omitted")
	}
}

// ExportUsersByFilter with empty filter pages through GetAllUsers until a
// short page; with keyword+group it uses the SearchUsers path.
func TestExportUsersByFilterPaging(t *testing.T) {
	truncateTables(t)
	for i := 0; i < 250; i++ {
		insertOpsUser(t, fmt.Sprintf("page-user-%03d", i), 0, func(u *User) {
			u.CreatedAt = int64(1700000000 + i)
		})
	}

	all, err := ExportUsersByFilter("", "")
	require.NoError(t, err)
	require.Len(t, all, 250)

	// LIKE %page-user-1% matches page-user-100..199 exactly (100 users).
	kw, err := ExportUsersByFilter("page-user-1", "")
	require.NoError(t, err)
	require.Len(t, kw, 100)

	group, err := ExportUsersByFilter("", "default")
	require.NoError(t, err)
	require.Len(t, group, 250)
}

// ExportUsersByFilter is Unscoped like the admin users table (which shows
// soft-deleted rows with status=-1): deleted users are exported too.
func TestExportUsersByFilterIncludesSoftDeleted(t *testing.T) {
	truncateTables(t)
	u := insertOpsUser(t, "deleted-user", 0)
	require.NoError(t, DB.Delete(&User{}, u.Id).Error)

	all, err := ExportUsersByFilter("", "")
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "deleted-user", all[0].Username)
}

// ExportUsersByFilter returns nil, error when a batch fails, so the caller
// fails the request instead of emitting a truncated CSV.
func TestExportUsersByFilterBatchError(t *testing.T) {
	// The package's shared TestMain DB must not be corrupted, so swap in a
	// closed throwaway DB for the duration of this test.
	prev := DB
	defer func() { DB = prev }()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	DB = db

	_, err = ExportUsersByFilter("", "")
	require.Error(t, err)
}
