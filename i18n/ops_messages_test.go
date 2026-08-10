package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpsCampaignNotAccessibleMessages verifies that each ops campaign
// not-accessible message renders with complete per-locale text (no template
// placeholders left unrendered) through the same path the app uses
// (i18n.Translate, which backs common.ApiErrorI18n).
func TestOpsCampaignNotAccessibleMessages(t *testing.T) {
	require.NoError(t, Init())

	cases := []struct {
		name string
		lang string
		key  string
		want string
	}{
		{name: "en detail", lang: LangEn, key: MsgOpsCampaignNotAccessible, want: "No permission to view this campaign"},
		{name: "en stats", lang: LangEn, key: MsgOpsCampaignStatsNotAccessible, want: "No permission to view this campaign statistics"},
		{name: "zh-CN participants", lang: LangZhCN, key: MsgOpsCampaignParticipantsNotAccessible, want: "无权查看该活动参与记录"},
		{name: "zh-TW rewards", lang: LangZhTW, key: MsgOpsCampaignRewardsNotAccessible, want: "無權查看該活動獎勵記錄"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Translate(tc.lang, tc.key)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestOpsExportHeaderAndStatusMessages verifies the ops CSV export headers and
// user status labels render per locale (they back opsUserExportHeaders and
// formatOpsUserRow, which must never leak untranslated keys).
func TestOpsExportHeaderAndStatusMessages(t *testing.T) {
	require.NoError(t, Init())

	cases := []struct {
		name string
		lang string
		key  string
		want string
	}{
		{name: "en header username", lang: LangEn, key: MsgOpsExportHeaderUsername, want: "Username"},
		{name: "zh-CN header phone", lang: LangZhCN, key: MsgOpsExportHeaderPhone, want: "手机号"},
		{name: "zh-TW header inviter", lang: LangZhTW, key: MsgOpsExportHeaderInviterId, want: "邀請人 ID"},
		{name: "en status enabled", lang: LangEn, key: MsgOpsStatusEnabled, want: "Enabled"},
		{name: "zh-CN status disabled", lang: LangZhCN, key: MsgOpsStatusDisabled, want: "已禁用"},
		{name: "zh-TW status enabled", lang: LangZhTW, key: MsgOpsStatusEnabled, want: "已啟用"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Translate(tc.lang, tc.key)
			assert.Equal(t, tc.want, got)
		})
	}
}
