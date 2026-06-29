package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubsiteSlugValidation(t *testing.T) {
	slug, err := NormalizeSubsiteSlug("  Codex-Day  ")
	require.NoError(t, err)
	assert.Equal(t, "codex-day", slug)

	for _, raw := range []string{"ab", "-codex", "codex-", "codex_day", "codex--day", "dashboard"} {
		_, err := NormalizeSubsiteSlug(raw)
		assert.Error(t, err, "slug %q should be rejected", raw)
	}
}

func TestSubsiteAccessDecision(t *testing.T) {
	const now = int64(1000)

	enabled := Subsite{Status: SubsiteStatusEnabled}
	assert.True(t, enabled.AccessDecision(now, false).Allowed)
	assert.Equal(t, SubsiteRuntimeStatusEnabled, enabled.AccessDecision(now, false).Status)

	disabled := Subsite{Status: SubsiteStatusDisabled, DisabledReason: "maintenance"}
	disabledDecision := disabled.AccessDecision(now, false)
	assert.False(t, disabledDecision.Allowed)
	assert.Equal(t, SubsiteAccessCodeDisabled, disabledDecision.Code)

	draft := Subsite{Status: SubsiteStatusDraft}
	assert.False(t, draft.AccessDecision(now, false).Allowed)
	assert.Equal(t, SubsiteAccessCodeDraft, draft.AccessDecision(now, false).Code)
	assert.True(t, draft.AccessDecision(now, true).Allowed)

	notStarted := Subsite{Status: SubsiteStatusEnabled, StartsAt: now + 1}
	assert.Equal(t, SubsiteAccessCodeNotStarted, notStarted.AccessDecision(now, false).Code)

	expired := Subsite{Status: SubsiteStatusEnabled, EndsAt: now - 1}
	assert.Equal(t, SubsiteAccessCodeExpired, expired.AccessDecision(now, false).Code)
}

func TestSubsiteMemberRBAC(t *testing.T) {
	assert.True(t, SubsiteRoleCanManage(SubsiteMemberRoleOwner))
	assert.True(t, SubsiteRoleCanManage(SubsiteMemberRoleAdmin))
	assert.False(t, SubsiteRoleCanManage(SubsiteMemberRoleMember))
	assert.True(t, SubsiteRoleCanOwn(SubsiteMemberRoleOwner))
	assert.False(t, SubsiteRoleCanOwn(SubsiteMemberRoleAdmin))

	member := SubsiteMember{Role: SubsiteMemberRoleAdmin, Status: SubsiteMemberStatusActive}
	assert.True(t, member.CanAccess())
	assert.True(t, member.CanManage())

	disabled := SubsiteMember{Role: SubsiteMemberRoleOwner, Status: SubsiteMemberStatusDisabled}
	assert.False(t, disabled.CanAccess())
	assert.False(t, disabled.CanManage())
}

func TestSubsiteRegistrationPolicy(t *testing.T) {
	open := Subsite{RegistrationPolicy: SubsiteRegistrationPolicyOpen}
	require.NoError(t, open.RegistrationAllowed("", ""))

	closed := Subsite{RegistrationPolicy: SubsiteRegistrationPolicyClosed}
	assert.ErrorContains(t, closed.RegistrationAllowed("user@example.com", ""), "closed")

	invite := Subsite{RegistrationPolicy: SubsiteRegistrationPolicyInvite, InviteCode: "let-me-in"}
	assert.ErrorContains(t, invite.RegistrationAllowed("user@example.com", "wrong"), "invite")
	require.NoError(t, invite.RegistrationAllowed("user@example.com", "let-me-in"))

	whitelist := Subsite{EmailDomainWhitelist: "example.com, @quantumnous.com"}
	require.NoError(t, whitelist.RegistrationAllowed("member@example.com", ""))
	require.NoError(t, whitelist.RegistrationAllowed("member@quantumnous.com", ""))
	assert.ErrorContains(t, whitelist.RegistrationAllowed("member@blocked.com", ""), "domain")
	assert.ErrorContains(t, whitelist.RegistrationAllowed("", ""), "domain")
}

func TestSubsiteModelsAutoMigrate(t *testing.T) {
	require.True(t, DB.Migrator().HasTable(&Subsite{}))
	require.True(t, DB.Migrator().HasTable(&SubsiteMember{}))
	require.True(t, DB.Migrator().HasTable(&SubsiteQuotaPolicy{}))
	require.True(t, DB.Migrator().HasTable(&SubsiteQuotaCounter{}))

	for _, item := range []struct {
		model any
		name  string
	}{
		{&Channel{}, "channels"},
		{&Token{}, "tokens"},
		{&Log{}, "logs"},
		{&BillingEvent{}, "billing_events"},
		{&RequestCaptureRecord{}, "request_capture_records"},
		{&RequestDiagnosticReport{}, "request_diagnostic_reports"},
	} {
		require.True(t, DB.Migrator().HasColumn(item.model, "subsite_id"), "%s should have subsite_id", item.name)
	}
}

func TestCreateSubsiteAndMember(t *testing.T) {
	truncateTables(t)

	subsite := &Subsite{
		Slug:      " Codex-Demo ",
		Name:      "Codex Demo",
		Status:    SubsiteStatusEnabled,
		CreatedBy: 1,
	}
	require.NoError(t, CreateSubsite(subsite))
	require.NotZero(t, subsite.Id)
	assert.Equal(t, "codex-demo", subsite.Slug)

	found, err := GetSubsiteBySlug("CODEX-DEMO")
	require.NoError(t, err)
	assert.Equal(t, subsite.Id, found.Id)

	member := &SubsiteMember{
		SubsiteId: subsite.Id,
		UserId:    7,
		Role:      SubsiteMemberRoleOwner,
	}
	require.NoError(t, CreateSubsiteMember(member))
	require.NotZero(t, member.JoinedAt)

	foundMember, err := GetSubsiteMember(subsite.Id, 7)
	require.NoError(t, err)
	assert.True(t, foundMember.CanManage())
}
