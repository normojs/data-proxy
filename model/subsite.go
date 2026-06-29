package model

import (
	"errors"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	SubsiteStatusDraft    = "draft"
	SubsiteStatusEnabled  = "enabled"
	SubsiteStatusDisabled = "disabled"

	SubsiteRuntimeStatusDraft      = "draft"
	SubsiteRuntimeStatusEnabled    = "enabled"
	SubsiteRuntimeStatusDisabled   = "disabled"
	SubsiteRuntimeStatusNotStarted = "not_started"
	SubsiteRuntimeStatusExpired    = "expired"

	SubsiteAccessCodeDisabled    = "subsite_disabled"
	SubsiteAccessCodeDraft       = "subsite_draft"
	SubsiteAccessCodeNotStarted  = "subsite_not_started"
	SubsiteAccessCodeExpired     = "subsite_expired"
	SubsiteAccessCodeNotFound    = "subsite_not_found"
	SubsiteAccessCodeAPINotReady = "subsite_api_not_ready"
	SubsiteAccessCodeTokenScope  = "subsite_token_scope_mismatch"
	SubsiteAccessCodeQuota       = "subsite_quota_exceeded"
	SubsiteAccessCodeUserQuota   = "subsite_user_quota_exceeded"
	SubsiteAccessCodeRateLimited = "subsite_rate_limited"

	SubsiteRegistrationPolicyOpen   = "open"
	SubsiteRegistrationPolicyInvite = "invite"
	SubsiteRegistrationPolicyClosed = "closed"

	SubsiteMemberRoleOwner  = "owner"
	SubsiteMemberRoleAdmin  = "admin"
	SubsiteMemberRoleMember = "member"

	SubsiteMemberStatusActive   = "active"
	SubsiteMemberStatusDisabled = "disabled"

	SubsiteCounterScopeSite = "site"
	SubsiteCounterScopeUser = "user"

	SubsiteCounterWindowDaily   = "daily"
	SubsiteCounterWindowRolling = "rolling"
)

var (
	subsiteSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

	reservedSubsiteSlugs = map[string]struct{}{
		"admin":     {},
		"api":       {},
		"assets":    {},
		"auth":      {},
		"dashboard": {},
		"docs":      {},
		"login":     {},
		"oauth":     {},
		"register":  {},
		"s":         {},
		"static":    {},
		"status":    {},
		"system":    {},
		"v1":        {},
	}
)

type Subsite struct {
	Id                   int64          `json:"id" gorm:"primaryKey"`
	Slug                 string         `json:"slug" gorm:"type:varchar(64);not null;uniqueIndex"`
	Name                 string         `json:"name" gorm:"type:varchar(128);not null"`
	Title                string         `json:"title" gorm:"type:varchar(128);not null;default:''"`
	LogoURL              string         `json:"logo_url" gorm:"type:varchar(512);not null;default:''"`
	FaviconURL           string         `json:"favicon_url" gorm:"type:varchar(512);not null;default:''"`
	ThemeColor           string         `json:"theme_color" gorm:"type:varchar(32);not null;default:''"`
	Status               string         `json:"status" gorm:"type:varchar(32);not null;default:'draft';index"`
	DisabledReason       string         `json:"disabled_reason" gorm:"type:varchar(512);not null;default:''"`
	AnnouncementIcon     string         `json:"announcement_icon" gorm:"type:varchar(128);not null;default:''"`
	AnnouncementTitle    string         `json:"announcement_title" gorm:"type:varchar(128);not null;default:''"`
	AnnouncementBody     string         `json:"announcement_body" gorm:"type:text"`
	AnnouncementURL      string         `json:"announcement_url" gorm:"type:varchar(512);not null;default:''"`
	ContactURL           string         `json:"contact_url" gorm:"type:varchar(512);not null;default:''"`
	RegistrationPolicy   string         `json:"registration_policy" gorm:"type:varchar(32);not null;default:'open'"`
	InviteCode           string         `json:"invite_code" gorm:"type:varchar(128);not null;default:''"`
	EmailDomainWhitelist string         `json:"email_domain_whitelist" gorm:"type:text"`
	StartsAt             int64          `json:"starts_at" gorm:"bigint;not null;default:0;index"`
	EndsAt               int64          `json:"ends_at" gorm:"bigint;not null;default:0;index"`
	CreatedBy            int            `json:"created_by" gorm:"not null;default:0;index"`
	CreatedAt            int64          `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt            int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt            gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Subsite) TableName() string {
	return "subsites"
}

func (subsite *Subsite) BeforeSave(tx *gorm.DB) error {
	slug, err := NormalizeSubsiteSlug(subsite.Slug)
	if err != nil {
		return err
	}
	subsite.Slug = slug
	subsite.Status = NormalizeSubsiteStatus(subsite.Status)
	subsite.RegistrationPolicy = NormalizeSubsiteRegistrationPolicy(subsite.RegistrationPolicy)
	subsite.Name = strings.TrimSpace(subsite.Name)
	if subsite.Name == "" {
		return errors.New("subsite name is required")
	}
	return nil
}

type SubsiteMember struct {
	Id        int64  `json:"id" gorm:"primaryKey"`
	SubsiteId int64  `json:"subsite_id" gorm:"not null;uniqueIndex:idx_subsite_members_site_user,priority:1;index"`
	UserId    int    `json:"user_id" gorm:"not null;uniqueIndex:idx_subsite_members_site_user,priority:2;index"`
	Role      string `json:"role" gorm:"type:varchar(32);not null;default:'member';index"`
	Status    string `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	JoinedAt  int64  `json:"joined_at" gorm:"bigint;not null;default:0;index"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SubsiteMember) TableName() string {
	return "subsite_members"
}

func (member *SubsiteMember) BeforeSave(tx *gorm.DB) error {
	member.Role = NormalizeSubsiteMemberRole(member.Role)
	member.Status = NormalizeSubsiteMemberStatus(member.Status)
	if member.JoinedAt == 0 {
		member.JoinedAt = common.GetTimestamp()
	}
	return nil
}

type SubsiteQuotaPolicy struct {
	Id                     int64 `json:"id" gorm:"primaryKey"`
	SubsiteId              int64 `json:"subsite_id" gorm:"not null;uniqueIndex"`
	SiteDailyQuota         int   `json:"site_daily_quota" gorm:"not null;default:0"`
	SiteWindowQuota        int   `json:"site_window_quota" gorm:"not null;default:0"`
	UserDailyQuota         int   `json:"user_daily_quota" gorm:"not null;default:0"`
	UserWindowQuota        int   `json:"user_window_quota" gorm:"not null;default:0"`
	SiteDailyRequestLimit  int   `json:"site_daily_request_limit" gorm:"not null;default:0"`
	SiteWindowRequestLimit int   `json:"site_window_request_limit" gorm:"not null;default:0"`
	UserDailyRequestLimit  int   `json:"user_daily_request_limit" gorm:"not null;default:0"`
	UserWindowRequestLimit int   `json:"user_window_request_limit" gorm:"not null;default:0"`
	SiteWindowSeconds      int64 `json:"site_window_seconds" gorm:"bigint;not null;default:0"`
	UserWindowSeconds      int64 `json:"user_window_seconds" gorm:"bigint;not null;default:0"`
	CreatedAt              int64 `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt              int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SubsiteQuotaPolicy) TableName() string {
	return "subsite_quota_policies"
}

type SubsiteQuotaCounter struct {
	Id           int64  `json:"id" gorm:"primaryKey"`
	SubsiteId    int64  `json:"subsite_id" gorm:"not null;uniqueIndex:idx_subsite_quota_counter_window,priority:1;index"`
	UserId       int    `json:"user_id" gorm:"not null;default:0;uniqueIndex:idx_subsite_quota_counter_window,priority:2;index"`
	Scope        string `json:"scope" gorm:"type:varchar(16);not null;uniqueIndex:idx_subsite_quota_counter_window,priority:3;index"`
	WindowType   string `json:"window_type" gorm:"type:varchar(16);not null;uniqueIndex:idx_subsite_quota_counter_window,priority:4;index"`
	WindowStart  int64  `json:"window_start" gorm:"bigint;not null;uniqueIndex:idx_subsite_quota_counter_window,priority:5;index"`
	WindowEnd    int64  `json:"window_end" gorm:"bigint;not null;default:0;index"`
	UsedQuota    int    `json:"used_quota" gorm:"not null;default:0"`
	RequestCount int    `json:"request_count" gorm:"not null;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SubsiteQuotaCounter) TableName() string {
	return "subsite_quota_counters"
}

func (counter *SubsiteQuotaCounter) BeforeSave(tx *gorm.DB) error {
	counter.Scope = NormalizeSubsiteCounterScope(counter.Scope)
	counter.WindowType = NormalizeSubsiteCounterWindowType(counter.WindowType)
	return nil
}

type SubsiteAccessDecision struct {
	Allowed bool
	Status  string
	Code    string
	Message string
}

func NormalizeSubsiteSlug(slug string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !subsiteSlugPattern.MatchString(slug) {
		return "", errors.New("subsite slug must be 3-64 chars and contain only lowercase letters, numbers, and hyphens")
	}
	if strings.Contains(slug, "--") {
		return "", errors.New("subsite slug cannot contain consecutive hyphens")
	}
	if IsReservedSubsiteSlug(slug) {
		return "", errors.New("subsite slug is reserved")
	}
	return slug, nil
}

func IsReservedSubsiteSlug(slug string) bool {
	_, ok := reservedSubsiteSlugs[strings.ToLower(strings.TrimSpace(slug))]
	return ok
}

func NormalizeSubsiteStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SubsiteStatusEnabled:
		return SubsiteStatusEnabled
	case SubsiteStatusDisabled:
		return SubsiteStatusDisabled
	default:
		return SubsiteStatusDraft
	}
}

func NormalizeSubsiteRegistrationPolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case SubsiteRegistrationPolicyInvite:
		return SubsiteRegistrationPolicyInvite
	case SubsiteRegistrationPolicyClosed:
		return SubsiteRegistrationPolicyClosed
	default:
		return SubsiteRegistrationPolicyOpen
	}
}

func (subsite *Subsite) RuntimeStatus(now int64) string {
	if subsite == nil {
		return SubsiteRuntimeStatusDisabled
	}
	switch NormalizeSubsiteStatus(subsite.Status) {
	case SubsiteStatusDraft:
		return SubsiteRuntimeStatusDraft
	case SubsiteStatusDisabled:
		return SubsiteRuntimeStatusDisabled
	}
	if subsite.StartsAt > 0 && now < subsite.StartsAt {
		return SubsiteRuntimeStatusNotStarted
	}
	if subsite.EndsAt > 0 && now > subsite.EndsAt {
		return SubsiteRuntimeStatusExpired
	}
	return SubsiteRuntimeStatusEnabled
}

func (subsite *Subsite) AccessDecision(now int64, preview bool) SubsiteAccessDecision {
	status := subsite.RuntimeStatus(now)
	if preview && status == SubsiteRuntimeStatusDraft {
		return SubsiteAccessDecision{Allowed: true, Status: status}
	}
	switch status {
	case SubsiteRuntimeStatusEnabled:
		return SubsiteAccessDecision{Allowed: true, Status: status}
	case SubsiteRuntimeStatusDraft:
		return SubsiteAccessDecision{
			Status:  status,
			Code:    SubsiteAccessCodeDraft,
			Message: "Subsite is not published yet",
		}
	case SubsiteRuntimeStatusNotStarted:
		return SubsiteAccessDecision{
			Status:  status,
			Code:    SubsiteAccessCodeNotStarted,
			Message: "Subsite access window has not started",
		}
	case SubsiteRuntimeStatusExpired:
		return SubsiteAccessDecision{
			Status:  status,
			Code:    SubsiteAccessCodeExpired,
			Message: "Subsite access window has ended",
		}
	default:
		return SubsiteAccessDecision{
			Status:  status,
			Code:    SubsiteAccessCodeDisabled,
			Message: "Subsite is currently disabled",
		}
	}
}

func (subsite *Subsite) RegistrationAllowed(email string, inviteCode string) error {
	if subsite == nil {
		return errors.New("subsite not found")
	}
	switch NormalizeSubsiteRegistrationPolicy(subsite.RegistrationPolicy) {
	case SubsiteRegistrationPolicyClosed:
		return errors.New("subsite registration is closed")
	case SubsiteRegistrationPolicyInvite:
		if strings.TrimSpace(subsite.InviteCode) == "" || strings.TrimSpace(inviteCode) != strings.TrimSpace(subsite.InviteCode) {
			return errors.New("invalid subsite invite code")
		}
	}
	if !subsite.EmailDomainAllowed(email) {
		return errors.New("email domain is not allowed for this subsite")
	}
	return nil
}

func (subsite *Subsite) EmailDomainAllowed(email string) bool {
	if subsite == nil {
		return false
	}
	raw := strings.TrimSpace(subsite.EmailDomainWhitelist)
	if raw == "" {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(email[at+1:]))
	if domain == "" {
		return false
	}
	for _, allowed := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';' || r == ' '
	}) {
		allowed = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(allowed)), "@")
		if allowed == domain {
			return true
		}
	}
	return false
}

func NormalizeSubsiteMemberRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case SubsiteMemberRoleOwner:
		return SubsiteMemberRoleOwner
	case SubsiteMemberRoleAdmin:
		return SubsiteMemberRoleAdmin
	default:
		return SubsiteMemberRoleMember
	}
}

func NormalizeSubsiteMemberStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case SubsiteMemberStatusDisabled:
		return SubsiteMemberStatusDisabled
	default:
		return SubsiteMemberStatusActive
	}
}

func SubsiteRoleCanManage(role string) bool {
	role = NormalizeSubsiteMemberRole(role)
	return role == SubsiteMemberRoleOwner || role == SubsiteMemberRoleAdmin
}

func SubsiteRoleCanOwn(role string) bool {
	return NormalizeSubsiteMemberRole(role) == SubsiteMemberRoleOwner
}

func (member *SubsiteMember) CanAccess() bool {
	return member != nil && NormalizeSubsiteMemberStatus(member.Status) == SubsiteMemberStatusActive
}

func (member *SubsiteMember) CanManage() bool {
	return member.CanAccess() && SubsiteRoleCanManage(member.Role)
}

func NormalizeSubsiteCounterScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case SubsiteCounterScopeUser:
		return SubsiteCounterScopeUser
	default:
		return SubsiteCounterScopeSite
	}
}

func NormalizeSubsiteCounterWindowType(windowType string) string {
	switch strings.ToLower(strings.TrimSpace(windowType)) {
	case SubsiteCounterWindowRolling:
		return SubsiteCounterWindowRolling
	default:
		return SubsiteCounterWindowDaily
	}
}

func CreateSubsite(subsite *Subsite) error {
	return DB.Create(subsite).Error
}

func GetSubsiteBySlug(slug string) (*Subsite, error) {
	normalized, err := NormalizeSubsiteSlug(slug)
	if err != nil {
		return nil, err
	}
	var subsite Subsite
	if err := DB.Where("slug = ?", normalized).First(&subsite).Error; err != nil {
		return nil, err
	}
	return &subsite, nil
}

func CreateSubsiteMember(member *SubsiteMember) error {
	return DB.Create(member).Error
}

func GetSubsiteMember(subsiteId int64, userId int) (*SubsiteMember, error) {
	var member SubsiteMember
	if err := DB.Where("subsite_id = ? AND user_id = ?", subsiteId, userId).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}
