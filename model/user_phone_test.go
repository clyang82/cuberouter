package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedPhoneUser(t *testing.T, id int, phone string) *User {
	t.Helper()
	username := fmt.Sprintf("phone-user-%d", id)
	user := &User{
		Id:          id,
		Username:    username,
		Password:    "x",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     fmt.Sprintf("aff-%d", id), // uniqueIndex — must be unique per user
		Phone:       phone,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestUserPhonePersistedViaEditWithTx(t *testing.T) {
	user := seedPhoneUser(t, 6001, "13800001111")

	edited := &User{Id: user.Id, Username: user.Username, Phone: "13900002222"}
	require.NoError(t, edited.EditWithTx(DB, false))

	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	assert.Equal(t, "13900002222", reloaded.Phone)

	// Map-based update can also clear the phone.
	cleared := &User{Id: user.Id, Username: user.Username, Phone: ""}
	require.NoError(t, cleared.EditWithTx(DB, false))
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	assert.Equal(t, "", reloaded.Phone)
}

func TestUserPhoneUpdateWithTxKeepsNonEmpty(t *testing.T) {
	user := seedPhoneUser(t, 6002, "")

	updated := &User{Id: user.Id, Username: user.Username, Phone: "13700003333"}
	require.NoError(t, updated.UpdateWithTx(DB, false))

	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	assert.Equal(t, "13700003333", reloaded.Phone)

	// Struct-based Updates skip zero values: attempting to clear via UpdateWithTx
	// preserves the existing phone. Documented behavior (same as the source repo);
	// clearing is only possible via EditWithTx.
	attemptClear := &User{Id: user.Id, Username: user.Username, Phone: ""}
	require.NoError(t, attemptClear.UpdateWithTx(DB, false))
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	assert.Equal(t, "13700003333", reloaded.Phone)
}
