package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetPublicSubsiteReturnsRuntimeAccess(t *testing.T) {
	setupSubsiteControllerTestDB(t)

	now := common.GetTimestamp()
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:              "open-site",
		Name:              "Open Site",
		Title:             "Open Console",
		Status:            model.SubsiteStatusEnabled,
		AnnouncementTitle: "Hello",
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:           "closed-site",
		Name:           "Closed Site",
		Status:         model.SubsiteStatusDisabled,
		DisabledReason: "maintenance",
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:   "ended-site",
		Name:   "Ended Site",
		Status: model.SubsiteStatusEnabled,
		EndsAt: now - 1,
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:     "soon-site",
		Name:     "Soon Site",
		Status:   model.SubsiteStatusEnabled,
		StartsAt: now + 3600,
	}))

	engine := gin.New()
	engine.GET("/api/subsites/:slug/public", GetPublicSubsite)

	open := performSubsitePublicRequest(engine, "open-site")
	require.Equal(t, http.StatusOK, open.Code, open.Body.String())
	openBody := decodeJSONBody(t, open)
	require.Equal(t, true, nestedValue(openBody, "data", "access", "allowed"))
	require.Equal(t, model.SubsiteRuntimeStatusEnabled, nestedValue(openBody, "data", "runtime_status"))
	require.Equal(t, "Hello", nestedValue(openBody, "data", "announcement_title"))

	closed := performSubsitePublicRequest(engine, "closed-site")
	require.Equal(t, http.StatusOK, closed.Code, closed.Body.String())
	closedBody := decodeJSONBody(t, closed)
	require.Equal(t, false, nestedValue(closedBody, "data", "access", "allowed"))
	require.Equal(t, model.SubsiteAccessCodeDisabled, nestedValue(closedBody, "data", "access", "code"))
	require.Equal(t, "maintenance", nestedValue(closedBody, "data", "disabled_reason"))

	ended := performSubsitePublicRequest(engine, "ended-site")
	require.Equal(t, http.StatusOK, ended.Code, ended.Body.String())
	endedBody := decodeJSONBody(t, ended)
	require.Equal(t, false, nestedValue(endedBody, "data", "access", "allowed"))
	require.Equal(t, model.SubsiteAccessCodeExpired, nestedValue(endedBody, "data", "access", "code"))

	soon := performSubsitePublicRequest(engine, "soon-site")
	require.Equal(t, http.StatusOK, soon.Code, soon.Body.String())
	soonBody := decodeJSONBody(t, soon)
	require.Equal(t, false, nestedValue(soonBody, "data", "access", "allowed"))
	require.Equal(t, model.SubsiteRuntimeStatusNotStarted, nestedValue(soonBody, "data", "runtime_status"))
	require.Equal(t, model.SubsiteAccessCodeNotStarted, nestedValue(soonBody, "data", "access", "code"))
}

func TestGetPublicSubsiteNotFound(t *testing.T) {
	setupSubsiteControllerTestDB(t)

	engine := gin.New()
	engine.GET("/api/subsites/:slug/public", GetPublicSubsite)

	recorder := performSubsitePublicRequest(engine, "missing-site")
	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), model.SubsiteAccessCodeNotFound)
}

func TestRegisterSubsiteUserCreatesMember(t *testing.T) {
	setupSubsiteControllerTestDB(t)
	constant.GenerateDefaultToken = true

	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:                 "join-site",
		Name:                 "Join Site",
		Status:               model.SubsiteStatusEnabled,
		RegistrationPolicy:   model.SubsiteRegistrationPolicyInvite,
		InviteCode:           "welcome",
		EmailDomainWhitelist: "example.com",
	}))

	engine := gin.New()
	engine.POST("/api/subsites/:slug/register", RegisterSubsiteUser)

	recorder := performSubsiteRegisterRequest(engine, "join-site", map[string]any{
		"username":    "alice",
		"password":    "password123",
		"email":       "alice@example.com",
		"invite_code": "welcome",
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := decodeJSONBody(t, recorder)
	require.Equal(t, true, body["success"])

	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "alice").First(&user).Error)
	require.NotZero(t, user.Id)
	require.Equal(t, "alice@example.com", user.Email)

	subsite, err := model.GetSubsiteBySlug("join-site")
	require.NoError(t, err)
	member, err := model.GetSubsiteMember(subsite.Id, user.Id)
	require.NoError(t, err)
	require.Equal(t, model.SubsiteMemberRoleMember, member.Role)
	require.True(t, member.CanAccess())

	var token model.Token
	require.NoError(t, model.DB.Where("subsite_id = ? AND user_id = ?", subsite.Id, user.Id).First(&token).Error)
	require.NotZero(t, token.Id)
	require.Contains(t, token.Name, "Join Site")

	var mainTokenCount int64
	require.NoError(t, model.DB.Model(&model.Token{}).Where("subsite_id = 0 AND user_id = ?", user.Id).Count(&mainTokenCount).Error)
	require.Zero(t, mainTokenCount)
}

func TestRegisterSubsiteUserRejectsPolicyViolations(t *testing.T) {
	setupSubsiteControllerTestDB(t)

	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:               "closed-reg",
		Name:               "Closed Registration",
		Status:             model.SubsiteStatusEnabled,
		RegistrationPolicy: model.SubsiteRegistrationPolicyClosed,
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:               "invite-reg",
		Name:               "Invite Registration",
		Status:             model.SubsiteStatusEnabled,
		RegistrationPolicy: model.SubsiteRegistrationPolicyInvite,
		InviteCode:         "secret",
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:                 "domain-reg",
		Name:                 "Domain Registration",
		Status:               model.SubsiteStatusEnabled,
		EmailDomainWhitelist: "example.com",
	}))

	engine := gin.New()
	engine.POST("/api/subsites/:slug/register", RegisterSubsiteUser)

	closed := performSubsiteRegisterRequest(engine, "closed-reg", map[string]any{
		"username": "closeduser",
		"password": "password123",
	})
	require.Equal(t, http.StatusOK, closed.Code, closed.Body.String())
	require.Equal(t, false, decodeJSONBody(t, closed)["success"])
	require.Contains(t, closed.Body.String(), "closed")

	invite := performSubsiteRegisterRequest(engine, "invite-reg", map[string]any{
		"username":    "inviteuser",
		"password":    "password123",
		"invite_code": "wrong",
	})
	require.Equal(t, http.StatusOK, invite.Code, invite.Body.String())
	require.Equal(t, false, decodeJSONBody(t, invite)["success"])
	require.Contains(t, invite.Body.String(), "invite")

	domain := performSubsiteRegisterRequest(engine, "domain-reg", map[string]any{
		"username": "domainuser",
		"password": "password123",
		"email":    "domain@blocked.com",
	})
	require.Equal(t, http.StatusOK, domain.Code, domain.Body.String())
	require.Equal(t, false, decodeJSONBody(t, domain)["success"])
	require.Contains(t, domain.Body.String(), "domain")
}

func TestGetSubsiteSelfMember(t *testing.T) {
	setupSubsiteControllerTestDB(t)

	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:   "member-site",
		Name:   "Member Site",
		Status: model.SubsiteStatusEnabled,
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:   "closed-member-site",
		Name:   "Closed Member Site",
		Status: model.SubsiteStatusDisabled,
	}))
	memberSite, err := model.GetSubsiteBySlug("member-site")
	require.NoError(t, err)
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{
		SubsiteId: memberSite.Id,
		UserId:    7,
		Role:      model.SubsiteMemberRoleMember,
		Status:    model.SubsiteMemberStatusActive,
	}))
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{
		SubsiteId: memberSite.Id,
		UserId:    8,
		Role:      model.SubsiteMemberRoleMember,
		Status:    model.SubsiteMemberStatusDisabled,
	}))

	engine := gin.New()
	engine.GET("/api/subsites/:slug/member/self", func(c *gin.Context) {
		if userId := c.GetHeader("X-Test-User"); userId == "8" {
			c.Set("id", 8)
		} else if userId == "9" {
			c.Set("id", 9)
		} else {
			c.Set("id", 7)
		}
		GetSubsiteSelfMember(c)
	})

	active := performSubsiteSelfMemberRequest(engine, "member-site", "7")
	require.Equal(t, http.StatusOK, active.Code, active.Body.String())
	activeBody := decodeJSONBody(t, active)
	require.Equal(t, true, activeBody["success"])
	require.Equal(t, true, nestedValue(activeBody, "data", "can_access"))

	disabled := performSubsiteSelfMemberRequest(engine, "member-site", "8")
	require.Equal(t, http.StatusOK, disabled.Code, disabled.Body.String())
	disabledBody := decodeJSONBody(t, disabled)
	require.Equal(t, true, disabledBody["success"])
	require.Equal(t, false, nestedValue(disabledBody, "data", "can_access"))

	missing := performSubsiteSelfMemberRequest(engine, "member-site", "9")
	require.Equal(t, http.StatusOK, missing.Code, missing.Body.String())
	require.Equal(t, false, decodeJSONBody(t, missing)["success"])

	closed := performSubsiteSelfMemberRequest(engine, "closed-member-site", "7")
	require.Equal(t, http.StatusForbidden, closed.Code, closed.Body.String())
	require.Contains(t, closed.Body.String(), model.SubsiteAccessCodeDisabled)
}

func TestGetSubsiteDashboardScopesTokenQuotaAndLogs(t *testing.T) {
	setupSubsiteControllerTestDB(t)

	now := common.GetTimestamp()
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:              "site-a",
		Name:              "Site A",
		Status:            model.SubsiteStatusEnabled,
		AnnouncementTitle: "Read me",
	}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{
		Slug:   "site-b",
		Name:   "Site B",
		Status: model.SubsiteStatusEnabled,
	}))
	siteA, err := model.GetSubsiteBySlug("site-a")
	require.NoError(t, err)
	siteB, err := model.GetSubsiteBySlug("site-b")
	require.NoError(t, err)
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{
		SubsiteId: siteA.Id,
		UserId:    7,
		Role:      model.SubsiteMemberRoleMember,
		Status:    model.SubsiteMemberStatusActive,
	}))
	require.NoError(t, model.DB.Create(&model.Token{
		SubsiteId:    siteA.Id,
		UserId:       7,
		Name:         "Site A Key",
		Key:          "site-a-secret-key",
		Status:       common.TokenStatusEnabled,
		CreatedTime:  now - 100,
		AccessedTime: now - 10,
		ExpiredTime:  -1,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		SubsiteId:    siteB.Id,
		UserId:       7,
		Name:         "Site B Key",
		Key:          "site-b-secret-key",
		Status:       common.TokenStatusEnabled,
		CreatedTime:  now - 100,
		AccessedTime: now - 10,
		ExpiredTime:  -1,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:       7,
		Name:         "Main Key",
		Key:          "main-secret-key",
		Status:       common.TokenStatusEnabled,
		CreatedTime:  now - 100,
		AccessedTime: now - 10,
		ExpiredTime:  -1,
	}).Error)
	dailyStart := utcDayStart(now)
	require.NoError(t, model.DB.Create(&model.SubsiteQuotaPolicy{
		SubsiteId:              siteA.Id,
		SiteDailyQuota:         1000,
		SiteWindowQuota:        400,
		UserDailyQuota:         300,
		UserWindowQuota:        120,
		SiteDailyRequestLimit:  50,
		SiteWindowRequestLimit: 20,
		UserDailyRequestLimit:  10,
		UserWindowRequestLimit: 5,
		SiteWindowSeconds:      3600,
		UserWindowSeconds:      1800,
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubsiteQuotaCounter{
		SubsiteId:    siteA.Id,
		UserId:       0,
		Scope:        model.SubsiteCounterScopeSite,
		WindowType:   model.SubsiteCounterWindowDaily,
		WindowStart:  dailyStart,
		WindowEnd:    dailyStart + 86400,
		UsedQuota:    250,
		RequestCount: 8,
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubsiteQuotaCounter{
		SubsiteId:    siteA.Id,
		UserId:       7,
		Scope:        model.SubsiteCounterScopeUser,
		WindowType:   model.SubsiteCounterWindowDaily,
		WindowStart:  dailyStart,
		WindowEnd:    dailyStart + 86400,
		UsedQuota:    90,
		RequestCount: 3,
	}).Error)
	require.NoError(t, model.DB.Create(&model.SubsiteQuotaCounter{
		SubsiteId:    siteA.Id,
		UserId:       7,
		Scope:        model.SubsiteCounterScopeUser,
		WindowType:   model.SubsiteCounterWindowRolling,
		WindowStart:  now - 60,
		WindowEnd:    now + 1740,
		UsedQuota:    40,
		RequestCount: 2,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		SubsiteId:        siteA.Id,
		UserId:           7,
		Username:         "alice",
		CreatedAt:        now - 60,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o-mini",
		PromptTokens:     11,
		CompletionTokens: 13,
		Quota:            24,
		UseTime:          2,
		Other:            `{"cache_tokens":5,"reasoning_tokens":7}`,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		SubsiteId: siteA.Id,
		UserId:    7,
		Username:  "alice",
		CreatedAt: now - 30,
		Type:      model.LogTypeError,
		ModelName: "gpt-4o-mini",
		Content:   "upstream failed",
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		SubsiteId:        siteB.Id,
		UserId:           7,
		Username:         "alice",
		CreatedAt:        now - 20,
		Type:             model.LogTypeConsume,
		ModelName:        "other",
		PromptTokens:     100,
		CompletionTokens: 100,
		Quota:            200,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		SubsiteId:        siteA.Id,
		UserId:           8,
		Username:         "bob",
		CreatedAt:        now - 10,
		Type:             model.LogTypeConsume,
		ModelName:        "other-user",
		PromptTokens:     100,
		CompletionTokens: 100,
		Quota:            200,
	}).Error)

	engine := gin.New()
	engine.GET("/api/subsites/:slug/dashboard", testSubsiteUser(7, "alice"), GetSubsiteDashboard)

	recorder := performSubsiteAuthenticatedRequest(engine, http.MethodGet, "/api/subsites/site-a/dashboard", "7")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	body := decodeJSONBody(t, recorder)
	require.Equal(t, true, body["success"])
	require.Equal(t, "https://proxy.example/s/site-a/v1", nestedValue(body, "data", "base_url"))
	require.Equal(t, "Site A Key", nestedValue(body, "data", "token", "name"))
	require.Nil(t, nestedValue(body, "data", "token", "key"))
	require.NotContains(t, recorder.Body.String(), "site-a-secret-key")
	require.NotContains(t, recorder.Body.String(), "site-b-secret-key")
	require.EqualValues(t, 2, nestedValue(body, "data", "stats_24h", "calls"))
	require.EqualValues(t, 24, nestedValue(body, "data", "stats_24h", "total_tokens"))
	require.EqualValues(t, 24, nestedValue(body, "data", "stats_24h", "quota"))
	require.EqualValues(t, 1000, nestedValue(body, "data", "quota", "site_daily_quota", "limit"))
	require.EqualValues(t, 250, nestedValue(body, "data", "quota", "site_daily_quota", "used"))
	require.EqualValues(t, 300, nestedValue(body, "data", "quota", "user_daily_quota", "limit"))
	require.EqualValues(t, 90, nestedValue(body, "data", "quota", "user_daily_quota", "used"))
	require.EqualValues(t, 120, nestedValue(body, "data", "quota", "user_window_quota", "limit"))
	require.EqualValues(t, 40, nestedValue(body, "data", "quota", "user_window_quota", "used"))
	logs := nestedValue(body, "data", "recent_logs").([]any)
	require.Len(t, logs, 2)
	require.NotContains(t, recorder.Body.String(), "other-user")
}

func TestSubsiteTokenLifecycleIsScoped(t *testing.T) {
	setupSubsiteControllerTestDB(t)

	require.NoError(t, model.CreateSubsite(&model.Subsite{Slug: "site-a", Name: "Site A", Status: model.SubsiteStatusEnabled}))
	require.NoError(t, model.CreateSubsite(&model.Subsite{Slug: "site-b", Name: "Site B", Status: model.SubsiteStatusEnabled}))
	siteA, err := model.GetSubsiteBySlug("site-a")
	require.NoError(t, err)
	siteB, err := model.GetSubsiteBySlug("site-b")
	require.NoError(t, err)
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{SubsiteId: siteA.Id, UserId: 7, Role: model.SubsiteMemberRoleMember, Status: model.SubsiteMemberStatusActive}))
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{SubsiteId: siteB.Id, UserId: 7, Role: model.SubsiteMemberRoleMember, Status: model.SubsiteMemberStatusActive}))
	require.NoError(t, model.DB.Create(&model.Token{
		SubsiteId:   siteB.Id,
		UserId:      7,
		Name:        "Site B Key",
		Key:         "site-b-stays",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:      7,
		Name:        "Main Key",
		Key:         "main-stays",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
	}).Error)

	engine := gin.New()
	engine.POST("/api/subsites/:slug/token", testSubsiteUser(7, "alice"), EnsureSubsiteToken)
	engine.POST("/api/subsites/:slug/token/key", testSubsiteUser(7, "alice"), GetSubsiteTokenKey)
	engine.POST("/api/subsites/:slug/token/rotate", testSubsiteUser(7, "alice"), RotateSubsiteToken)

	created := performSubsiteAuthenticatedRequest(engine, http.MethodPost, "/api/subsites/site-a/token", "7")
	require.Equal(t, http.StatusOK, created.Code, created.Body.String())
	createdBody := decodeJSONBody(t, created)
	require.Equal(t, true, nestedValue(createdBody, "data", "created"))
	firstKey, ok := nestedValue(createdBody, "data", "token", "key").(string)
	require.True(t, ok)
	require.NotEmpty(t, firstKey)

	var siteAToken model.Token
	require.NoError(t, model.DB.Where("subsite_id = ? AND user_id = ?", siteA.Id, 7).First(&siteAToken).Error)
	require.Equal(t, firstKey, siteAToken.Key)

	key := performSubsiteAuthenticatedRequest(engine, http.MethodPost, "/api/subsites/site-a/token/key", "7")
	require.Equal(t, http.StatusOK, key.Code, key.Body.String())
	require.Equal(t, firstKey, nestedValue(decodeJSONBody(t, key), "data", "key"))

	rotated := performSubsiteAuthenticatedRequest(engine, http.MethodPost, "/api/subsites/site-a/token/rotate", "7")
	require.Equal(t, http.StatusOK, rotated.Code, rotated.Body.String())
	rotatedKey := nestedValue(decodeJSONBody(t, rotated), "data", "token", "key").(string)
	require.NotEmpty(t, rotatedKey)
	require.NotEqual(t, firstKey, rotatedKey)

	var siteBToken model.Token
	require.NoError(t, model.DB.Where("subsite_id = ? AND user_id = ?", siteB.Id, 7).First(&siteBToken).Error)
	require.Equal(t, "site-b-stays", siteBToken.Key)
	var mainToken model.Token
	require.NoError(t, model.DB.Where("subsite_id = 0 AND user_id = ?", 7).First(&mainToken).Error)
	require.Equal(t, "main-stays", mainToken.Key)

	var manageLogs int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("subsite_id = ? AND user_id = ? AND type = ?", siteA.Id, 7, model.LogTypeManage).Count(&manageLogs).Error)
	require.EqualValues(t, 2, manageLogs)
}

func TestManagedSubsiteCreateListUpdateQuotaAndAudit(t *testing.T) {
	setupSubsiteControllerTestDB(t)
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 7, Username: "owner", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "owner-aff"},
		{Id: 99, Username: "root", Password: "password123", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, AffCode: "root-aff"},
	}).Error)

	engine := gin.New()
	engine.POST("/api/subsite-management/subsites", testManagedSubsiteUser(99, common.RoleAdminUser, "root"), CreateManagedSubsite)
	engine.GET("/api/subsite-management/subsites", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), ListManagedSubsites)
	engine.GET("/api/subsite-management/subsites/:id/activity", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), GetManagedSubsiteActivity)
	engine.PATCH("/api/subsite-management/subsites/:id", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), UpdateManagedSubsite)
	engine.PUT("/api/subsite-management/subsites/:id/quota-policy", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), UpsertManagedSubsiteQuotaPolicy)

	create := performJSONRequest(engine, http.MethodPost, "/api/subsite-management/subsites", map[string]any{
		"slug":          "managed-site",
		"name":          "Managed Site",
		"title":         "Managed Console",
		"status":        model.SubsiteStatusEnabled,
		"owner_user_id": 7,
		"quota_policy": map[string]any{
			"site_daily_quota": 100,
			"user_daily_quota": 10,
		},
	})
	require.Equal(t, http.StatusOK, create.Code, create.Body.String())
	createBody := decodeJSONBody(t, create)
	require.Equal(t, true, createBody["success"])
	require.Equal(t, "managed-site", nestedValue(createBody, "data", "subsite", "slug"))
	require.Equal(t, float64(100), nestedValue(createBody, "data", "quota_policy", "site_daily_quota"))

	var subsite model.Subsite
	require.NoError(t, model.DB.Where("slug = ?", "managed-site").First(&subsite).Error)
	member, err := model.GetSubsiteMember(subsite.Id, 7)
	require.NoError(t, err)
	require.Equal(t, model.SubsiteMemberRoleOwner, member.Role)
	var policy model.SubsiteQuotaPolicy
	require.NoError(t, model.DB.Where("subsite_id = ?", subsite.Id).First(&policy).Error)
	require.Equal(t, 100, policy.SiteDailyQuota)

	require.NoError(t, model.LOG_DB.Create(&model.Log{
		SubsiteId:        subsite.Id,
		UserId:           7,
		Username:         "owner",
		CreatedAt:        common.GetTimestamp(),
		Type:             model.LogTypeConsume,
		Content:          "managed usage",
		Quota:            33,
		PromptTokens:     3,
		CompletionTokens: 4,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		SubsiteId:        subsite.Id,
		UserId:           8,
		Username:         "other",
		CreatedAt:        common.GetTimestamp(),
		Type:             model.LogTypeError,
		Content:          "managed error",
		ModelName:        "gpt-error",
		Quota:            12,
		PromptTokens:     1,
		CompletionTokens: 2,
	}).Error)

	list := performJSONRequest(engine, http.MethodGet, "/api/subsite-management/subsites?page_size=10", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	listBody := decodeJSONBody(t, list)
	require.Equal(t, true, listBody["success"])
	require.Equal(t, float64(1), nestedValue(listBody, "data", "total"))
	items := nestedValue(listBody, "data", "items").([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.Equal(t, "owner", item["role"])
	require.Equal(t, true, item["can_manage"])
	require.Equal(t, float64(1), item["member_count"])
	require.Equal(t, float64(2), item["today_calls"])
	require.Equal(t, float64(45), item["today_quota"])

	activity := performJSONRequest(engine, http.MethodGet, "/api/subsite-management/subsites/"+strconv.FormatInt(subsite.Id, 10)+"/activity", nil)
	require.Equal(t, http.StatusOK, activity.Code, activity.Body.String())
	activityBody := decodeJSONBody(t, activity)
	require.Equal(t, true, activityBody["success"])
	require.Equal(t, float64(2), nestedValue(activityBody, "data", "stats_24h", "calls"))
	require.Equal(t, float64(1), nestedValue(activityBody, "data", "error_calls_24h"))
	require.Equal(t, float64(45), nestedValue(activityBody, "data", "stats_24h", "quota"))
	require.Equal(t, float64(10), nestedValue(activityBody, "data", "stats_24h", "total_tokens"))
	recentLogs := nestedValue(activityBody, "data", "recent_logs").([]any)
	require.Len(t, recentLogs, 2)
	require.Equal(t, "error", recentLogs[0].(map[string]any)["status"])

	update := performJSONRequest(engine, http.MethodPatch, "/api/subsite-management/subsites/"+strconv.FormatInt(subsite.Id, 10), map[string]any{
		"status":              model.SubsiteStatusDisabled,
		"disabled_reason":     "maintenance",
		"registration_policy": model.SubsiteRegistrationPolicyInvite,
		"invite_code":         "join",
	})
	require.Equal(t, http.StatusOK, update.Code, update.Body.String())
	updateBody := decodeJSONBody(t, update)
	require.Equal(t, true, updateBody["success"])
	require.Equal(t, model.SubsiteStatusDisabled, nestedValue(updateBody, "data", "subsite", "status"))
	require.Equal(t, "maintenance", nestedValue(updateBody, "data", "subsite", "disabled_reason"))

	quota := performJSONRequest(engine, http.MethodPut, "/api/subsite-management/subsites/"+strconv.FormatInt(subsite.Id, 10)+"/quota-policy", map[string]any{
		"site_daily_quota":          200,
		"user_daily_quota":          20,
		"site_window_seconds":       3600,
		"user_window_seconds":       1800,
		"site_window_request_limit": 50,
		"user_window_request_limit": 5,
	})
	require.Equal(t, http.StatusOK, quota.Code, quota.Body.String())
	quotaBody := decodeJSONBody(t, quota)
	require.Equal(t, true, quotaBody["success"])
	require.Equal(t, float64(200), nestedValue(quotaBody, "data", "site_daily_quota"))

	require.NoError(t, model.DB.Where("subsite_id = ?", subsite.Id).First(&policy).Error)
	require.Equal(t, 200, policy.SiteDailyQuota)
	require.Equal(t, int64(3600), policy.SiteWindowSeconds)

	var manageLogs []model.Log
	require.NoError(t, model.LOG_DB.Where("subsite_id = ? AND type = ?", subsite.Id, model.LogTypeManage).Order("id asc").Find(&manageLogs).Error)
	contents := make([]string, 0, len(manageLogs))
	for _, log := range manageLogs {
		contents = append(contents, log.Content)
	}
	require.Contains(t, contents, "created subsite")
	require.Contains(t, contents, "updated subsite quota policy")
	require.Contains(t, contents, "updated subsite settings")
	require.Contains(t, contents, "disabled subsite")
}

func TestManagedSubsiteMembersManageRolesStatusRemoveAndAudit(t *testing.T) {
	setupSubsiteControllerTestDB(t)
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 7, Username: "owner", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "owner-aff"},
		{Id: 8, Username: "operator", DisplayName: "Operator", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Email: "operator@example.com", AffCode: "operator-aff"},
		{Id: 9, Username: "viewer", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "viewer-aff"},
	}).Error)
	require.NoError(t, model.CreateSubsite(&model.Subsite{Slug: "member-admin", Name: "Member Admin", Status: model.SubsiteStatusEnabled}))
	subsite, err := model.GetSubsiteBySlug("member-admin")
	require.NoError(t, err)
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{SubsiteId: subsite.Id, UserId: 7, Role: model.SubsiteMemberRoleOwner, Status: model.SubsiteMemberStatusActive}))

	engine := gin.New()
	engine.GET("/api/subsite-management/subsites/:id/members", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), ListManagedSubsiteMembers)
	engine.PUT("/api/subsite-management/subsites/:id/members", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), UpsertManagedSubsiteMember)
	engine.DELETE("/api/subsite-management/subsites/:id/members/:user_id", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), DeleteManagedSubsiteMember)

	path := "/api/subsite-management/subsites/" + strconv.FormatInt(subsite.Id, 10) + "/members"
	list := performJSONRequest(engine, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	listBody := decodeJSONBody(t, list)
	require.Equal(t, true, listBody["success"])
	require.Len(t, nestedValue(listBody, "data").([]any), 1)

	addAdmin := performJSONRequest(engine, http.MethodPut, path, map[string]any{
		"user_id": 8,
		"role":    model.SubsiteMemberRoleAdmin,
		"status":  model.SubsiteMemberStatusActive,
	})
	require.Equal(t, http.StatusOK, addAdmin.Code, addAdmin.Body.String())
	addAdminBody := decodeJSONBody(t, addAdmin)
	require.Equal(t, true, addAdminBody["success"])
	require.Equal(t, "operator", nestedValue(addAdminBody, "data", "username"))
	require.Equal(t, model.SubsiteMemberRoleAdmin, nestedValue(addAdminBody, "data", "role"))
	require.Equal(t, true, nestedValue(addAdminBody, "data", "can_manage"))

	addDisabledMember := performJSONRequest(engine, http.MethodPut, path, map[string]any{
		"user_id": 9,
		"role":    model.SubsiteMemberRoleMember,
		"status":  model.SubsiteMemberStatusDisabled,
	})
	require.Equal(t, http.StatusOK, addDisabledMember.Code, addDisabledMember.Body.String())
	require.Equal(t, false, nestedValue(decodeJSONBody(t, addDisabledMember), "data", "can_access"))

	demoteLastOwner := performJSONRequest(engine, http.MethodPut, path, map[string]any{
		"user_id": 7,
		"role":    model.SubsiteMemberRoleAdmin,
		"status":  model.SubsiteMemberStatusActive,
	})
	require.Equal(t, http.StatusOK, demoteLastOwner.Code, demoteLastOwner.Body.String())
	require.Equal(t, false, decodeJSONBody(t, demoteLastOwner)["success"])

	promoteSecondOwner := performJSONRequest(engine, http.MethodPut, path, map[string]any{
		"user_id": 8,
		"role":    model.SubsiteMemberRoleOwner,
		"status":  model.SubsiteMemberStatusActive,
	})
	require.Equal(t, http.StatusOK, promoteSecondOwner.Code, promoteSecondOwner.Body.String())
	require.Equal(t, true, decodeJSONBody(t, promoteSecondOwner)["success"])

	demoteOriginalOwner := performJSONRequest(engine, http.MethodPut, path, map[string]any{
		"user_id": 7,
		"role":    model.SubsiteMemberRoleAdmin,
		"status":  model.SubsiteMemberStatusActive,
	})
	require.Equal(t, http.StatusOK, demoteOriginalOwner.Code, demoteOriginalOwner.Body.String())
	require.Equal(t, true, decodeJSONBody(t, demoteOriginalOwner)["success"])

	disableLastOwner := performJSONRequest(engine, http.MethodPut, path, map[string]any{
		"user_id": 8,
		"role":    model.SubsiteMemberRoleOwner,
		"status":  model.SubsiteMemberStatusDisabled,
	})
	require.Equal(t, http.StatusOK, disableLastOwner.Code, disableLastOwner.Body.String())
	require.Equal(t, false, decodeJSONBody(t, disableLastOwner)["success"])

	removeAdmin := performJSONRequest(engine, http.MethodDelete, path+"/7", nil)
	require.Equal(t, http.StatusOK, removeAdmin.Code, removeAdmin.Body.String())
	require.Equal(t, true, decodeJSONBody(t, removeAdmin)["success"])

	ownerEngine := gin.New()
	ownerEngine.GET("/api/subsite-management/subsites/:id/members", testManagedSubsiteUser(8, common.RoleCommonUser, "operator"), ListManagedSubsiteMembers)
	list = performJSONRequest(ownerEngine, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	members := nestedValue(decodeJSONBody(t, list), "data").([]any)
	require.Len(t, members, 2)
	require.NotContains(t, list.Body.String(), `"user_id":7`)

	var manageLogs []model.Log
	require.NoError(t, model.LOG_DB.Where("subsite_id = ? AND type = ?", subsite.Id, model.LogTypeManage).Order("id asc").Find(&manageLogs).Error)
	contents := make([]string, 0, len(manageLogs))
	for _, log := range manageLogs {
		contents = append(contents, log.Content)
	}
	require.Contains(t, contents, "updated subsite member")
	require.Contains(t, contents, "disabled subsite member")
	require.Contains(t, contents, "removed subsite member")
}

func TestManagedSubsiteChannelsManageModelsAndAudit(t *testing.T) {
	setupSubsiteControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 7, Username: "owner", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "owner-aff"}).Error)
	require.NoError(t, model.CreateSubsite(&model.Subsite{Slug: "channel-admin", Name: "Channel Admin", Status: model.SubsiteStatusEnabled}))
	subsite, err := model.GetSubsiteBySlug("channel-admin")
	require.NoError(t, err)
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{SubsiteId: subsite.Id, UserId: 7, Role: model.SubsiteMemberRoleOwner, Status: model.SubsiteMemberStatusActive}))

	engine := gin.New()
	engine.GET("/api/subsite-management/subsites/:id/channels", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), ListManagedSubsiteChannels)
	engine.POST("/api/subsite-management/subsites/:id/channels", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), CreateManagedSubsiteChannel)
	engine.GET("/api/subsite-management/subsites/:id/channels/:channel_id/upstream-models", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), GetManagedSubsiteChannelUpstreamModels)
	engine.POST("/api/subsite-management/subsites/:id/channels/:channel_id/models/sync", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), SyncManagedSubsiteChannelModels)
	engine.PATCH("/api/subsite-management/subsites/:id/channels/:channel_id", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), UpdateManagedSubsiteChannel)
	engine.DELETE("/api/subsite-management/subsites/:id/channels/:channel_id", testManagedSubsiteUser(7, common.RoleCommonUser, "owner"), DeleteManagedSubsiteChannel)

	path := "/api/subsite-management/subsites/" + strconv.FormatInt(subsite.Id, 10) + "/channels"
	list := performJSONRequest(engine, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Len(t, nestedValue(decodeJSONBody(t, list), "data").([]any), 0)

	create := performJSONRequest(engine, http.MethodPost, path, map[string]any{
		"name":   "OpenAI Event",
		"type":   constant.ChannelTypeOpenAI,
		"key":    "sk-subsite-channel",
		"models": "gpt-subsite-a, gpt-subsite-a, gpt-subsite-b",
		"model_display_names": map[string]any{
			"gpt-subsite-a": "Subsite Alpha",
			"gpt-subsite-b": "Subsite Beta",
			"gpt-hidden":    "Hidden",
		},
		"group":    "",
		"status":   common.ChannelStatusEnabled,
		"priority": 5,
		"weight":   10,
	})
	require.Equal(t, http.StatusOK, create.Code, create.Body.String())
	createBody := decodeJSONBody(t, create)
	require.Equal(t, true, createBody["success"])
	require.Equal(t, "gpt-subsite-a,gpt-subsite-b", nestedValue(createBody, "data", "models"))
	require.Equal(t, "default", nestedValue(createBody, "data", "group"))
	require.Equal(t, true, nestedValue(createBody, "data", "has_key"))
	createDisplayNames := nestedValue(createBody, "data", "model_display_names").(map[string]any)
	require.Equal(t, "Subsite Alpha", createDisplayNames["gpt-subsite-a"])
	require.Equal(t, "Subsite Beta", createDisplayNames["gpt-subsite-b"])
	require.NotContains(t, createDisplayNames, "gpt-hidden")
	channelId := int(nestedValue(createBody, "data", "id").(float64))
	var storedKey string
	require.NoError(t, model.DB.Table("channels").Select("key").Where("id = ?", channelId).Scan(&storedKey).Error)
	require.NotContains(t, storedKey, "sk-subsite-channel")
	require.True(t, strings.HasPrefix(storedKey, "enc:v1:"))

	require.True(t, model.IsModelEnabledForGroupsInSubsite("gpt-subsite-a", []string{"default"}, subsite.Id))
	require.False(t, model.IsModelEnabledForGroupsInSubsite("gpt-subsite-a", []string{"default"}, 0))

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer sk-subsite-channel", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-upstream-a"},{"id":"gpt-upstream-b"}]}`))
	}))
	defer upstream.Close()

	update := performJSONRequest(engine, http.MethodPatch, path+"/"+strconv.Itoa(channelId), map[string]any{
		"name":     "VIP Event",
		"type":     constant.ChannelTypeOpenAI,
		"base_url": upstream.URL,
		"models":   "gpt-subsite-c",
		"model_display_names": map[string]any{
			"gpt-subsite-c": "Subsite C",
			"gpt-subsite-a": "Old Alpha",
		},
		"group":  "vip",
		"status": common.ChannelStatusEnabled,
	})
	require.Equal(t, http.StatusOK, update.Code, update.Body.String())
	updateBody := decodeJSONBody(t, update)
	require.Equal(t, true, updateBody["success"])
	require.Equal(t, "gpt-subsite-c", nestedValue(updateBody, "data", "models"))
	require.Equal(t, "vip", nestedValue(updateBody, "data", "group"))
	require.Equal(t, true, nestedValue(updateBody, "data", "has_key"))
	updateDisplayNames := nestedValue(updateBody, "data", "model_display_names").(map[string]any)
	require.Equal(t, "Subsite C", updateDisplayNames["gpt-subsite-c"])
	require.NotContains(t, updateDisplayNames, "gpt-subsite-a")
	require.False(t, model.IsModelEnabledForGroupsInSubsite("gpt-subsite-a", []string{"default"}, subsite.Id))
	require.True(t, model.IsModelEnabledForGroupsInSubsite("gpt-subsite-c", []string{"vip"}, subsite.Id))

	fetchModels := performJSONRequest(engine, http.MethodGet, path+"/"+strconv.Itoa(channelId)+"/upstream-models", nil)
	require.Equal(t, http.StatusOK, fetchModels.Code, fetchModels.Body.String())
	require.Equal(t, true, decodeJSONBody(t, fetchModels)["success"])
	require.ElementsMatch(t, []any{"gpt-upstream-a", "gpt-upstream-b"}, nestedValue(decodeJSONBody(t, fetchModels), "data", "models").([]any))

	syncModels := performJSONRequest(engine, http.MethodPost, path+"/"+strconv.Itoa(channelId)+"/models/sync", nil)
	require.Equal(t, http.StatusOK, syncModels.Code, syncModels.Body.String())
	syncBody := decodeJSONBody(t, syncModels)
	require.Equal(t, true, syncBody["success"])
	require.Equal(t, "gpt-upstream-a,gpt-upstream-b", nestedValue(syncBody, "data", "models"))
	require.Empty(t, nestedValue(syncBody, "data", "model_display_names").(map[string]any))
	require.False(t, model.IsModelEnabledForGroupsInSubsite("gpt-subsite-c", []string{"vip"}, subsite.Id))
	require.True(t, model.IsModelEnabledForGroupsInSubsite("gpt-upstream-a", []string{"vip"}, subsite.Id))

	remove := performJSONRequest(engine, http.MethodDelete, path+"/"+strconv.Itoa(channelId), nil)
	require.Equal(t, http.StatusOK, remove.Code, remove.Body.String())
	require.Equal(t, true, decodeJSONBody(t, remove)["success"])
	require.False(t, model.IsModelEnabledForGroupsInSubsite("gpt-subsite-c", []string{"vip"}, subsite.Id))

	list = performJSONRequest(engine, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Len(t, nestedValue(decodeJSONBody(t, list), "data").([]any), 0)

	var manageLogs []model.Log
	require.NoError(t, model.LOG_DB.Where("subsite_id = ? AND type = ?", subsite.Id, model.LogTypeManage).Order("id asc").Find(&manageLogs).Error)
	contents := make([]string, 0, len(manageLogs))
	for _, log := range manageLogs {
		contents = append(contents, log.Content)
	}
	require.Contains(t, contents, "created subsite channel")
	require.Contains(t, contents, "updated subsite channel")
	require.Contains(t, contents, "synced subsite channel models")
	require.Contains(t, contents, "removed subsite channel")
}

func TestManagedSubsiteRejectsNonManager(t *testing.T) {
	setupSubsiteControllerTestDB(t)
	require.NoError(t, model.DB.Create(&[]model.User{
		{Id: 7, Username: "member", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "member-aff"},
		{Id: 8, Username: "owner", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "owner-aff"},
		{Id: 9, Username: "target", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "target-aff"},
	}).Error)
	require.NoError(t, model.CreateSubsite(&model.Subsite{Slug: "locked-site", Name: "Locked Site", Status: model.SubsiteStatusEnabled}))
	subsite, err := model.GetSubsiteBySlug("locked-site")
	require.NoError(t, err)
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{SubsiteId: subsite.Id, UserId: 8, Role: model.SubsiteMemberRoleOwner, Status: model.SubsiteMemberStatusActive}))
	require.NoError(t, model.CreateSubsiteMember(&model.SubsiteMember{SubsiteId: subsite.Id, UserId: 7, Role: model.SubsiteMemberRoleMember, Status: model.SubsiteMemberStatusActive}))

	engine := gin.New()
	engine.GET("/api/subsite-management/subsites", testManagedSubsiteUser(7, common.RoleCommonUser, "member"), ListManagedSubsites)
	engine.PATCH("/api/subsite-management/subsites/:id", testManagedSubsiteUser(7, common.RoleCommonUser, "member"), UpdateManagedSubsite)
	engine.PUT("/api/subsite-management/subsites/:id/members", testManagedSubsiteUser(7, common.RoleCommonUser, "member"), UpsertManagedSubsiteMember)
	engine.POST("/api/subsite-management/subsites/:id/channels", testManagedSubsiteUser(7, common.RoleCommonUser, "member"), CreateManagedSubsiteChannel)

	list := performJSONRequest(engine, http.MethodGet, "/api/subsite-management/subsites?page_size=10", nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	listBody := decodeJSONBody(t, list)
	require.Equal(t, true, listBody["success"])
	require.Equal(t, float64(0), nestedValue(listBody, "data", "total"))

	update := performJSONRequest(engine, http.MethodPatch, "/api/subsite-management/subsites/"+strconv.FormatInt(subsite.Id, 10), map[string]any{
		"name": "Should Not Change",
	})
	require.Equal(t, http.StatusOK, update.Code, update.Body.String())
	updateBody := decodeJSONBody(t, update)
	require.Equal(t, false, updateBody["success"])

	var unchanged model.Subsite
	require.NoError(t, model.DB.First(&unchanged, "id = ?", subsite.Id).Error)
	require.Equal(t, "Locked Site", unchanged.Name)

	memberUpdate := performJSONRequest(engine, http.MethodPut, "/api/subsite-management/subsites/"+strconv.FormatInt(subsite.Id, 10)+"/members", map[string]any{
		"user_id": 9,
		"role":    model.SubsiteMemberRoleAdmin,
		"status":  model.SubsiteMemberStatusActive,
	})
	require.Equal(t, http.StatusOK, memberUpdate.Code, memberUpdate.Body.String())
	require.Equal(t, false, decodeJSONBody(t, memberUpdate)["success"])

	var addedMembers int64
	require.NoError(t, model.DB.Model(&model.SubsiteMember{}).Where("subsite_id = ? AND user_id = ?", subsite.Id, 9).Count(&addedMembers).Error)
	require.EqualValues(t, 0, addedMembers)

	channelCreate := performJSONRequest(engine, http.MethodPost, "/api/subsite-management/subsites/"+strconv.FormatInt(subsite.Id, 10)+"/channels", map[string]any{
		"name":   "Should Not Add",
		"type":   constant.ChannelTypeOpenAI,
		"key":    "sk-blocked",
		"models": "gpt-blocked",
	})
	require.Equal(t, http.StatusOK, channelCreate.Code, channelCreate.Body.String())
	require.Equal(t, false, decodeJSONBody(t, channelCreate)["success"])

	var addedChannels int64
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("subsite_id = ?", subsite.Id).Count(&addedChannels).Error)
	require.EqualValues(t, 0, addedChannels)
}

func performSubsitePublicRequest(engine *gin.Engine, slug string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/subsites/"+slug+"/public", nil)
	engine.ServeHTTP(recorder, req)
	return recorder
}

func performSubsiteSelfMemberRequest(engine *gin.Engine, slug string, userId string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/subsites/"+slug+"/member/self", nil)
	req.Header.Set("X-Test-User", userId)
	engine.ServeHTTP(recorder, req)
	return recorder
}

func performSubsiteAuthenticatedRequest(engine *gin.Engine, method string, path string, userId string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "proxy.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Test-User", userId)
	engine.ServeHTTP(recorder, req)
	return recorder
}

func performSubsiteRegisterRequest(engine *gin.Engine, slug string, payload map[string]any) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/subsites/"+slug+"/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	return recorder
}

func performJSONRequest(engine *gin.Engine, method string, path string, payload map[string]any) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			panic(err)
		}
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, req)
	return recorder
}

func testSubsiteUser(defaultUserId int, username string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := defaultUserId
		if c.GetHeader("X-Test-User") == "8" {
			userId = 8
		}
		c.Set("id", userId)
		c.Set("username", username)
		c.Next()
	}
}

func testManagedSubsiteUser(userId int, role int, username string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("id", userId)
		c.Set("role", role)
		c.Set("username", username)
		c.Next()
	}
}

func utcDayStart(now int64) int64 {
	t := time.Unix(now, 0).UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix()
}

func setupSubsiteControllerTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalQuotaForNewUser := common.QuotaForNewUser
	originalRedisEnabled := common.RedisEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken

	common.UsingSQLite = true
	common.RedisEnabled = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	common.QuotaForNewUser = 0
	constant.GenerateDefaultToken = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.Subsite{},
		&model.SubsiteMember{},
		&model.SubsiteQuotaPolicy{},
		&model.SubsiteQuotaCounter{},
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Channel{},
		&model.Ability{},
	))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.QuotaForNewUser = originalQuotaForNewUser
		common.RedisEnabled = originalRedisEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})
}

func decodeJSONBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func nestedValue(body map[string]any, keys ...string) any {
	var current any = body
	for _, key := range keys {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = obj[key]
	}
	return current
}
