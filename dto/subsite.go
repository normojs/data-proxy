package dto

type SubsiteAccessInfo struct {
	Allowed bool   `json:"allowed"`
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PublicSubsite struct {
	Id                 int64             `json:"id"`
	Slug               string            `json:"slug"`
	Name               string            `json:"name"`
	Title              string            `json:"title"`
	LogoURL            string            `json:"logo_url"`
	FaviconURL         string            `json:"favicon_url"`
	ThemeColor         string            `json:"theme_color"`
	Status             string            `json:"status"`
	RuntimeStatus      string            `json:"runtime_status"`
	DisabledReason     string            `json:"disabled_reason"`
	AnnouncementIcon   string            `json:"announcement_icon"`
	AnnouncementTitle  string            `json:"announcement_title"`
	AnnouncementBody   string            `json:"announcement_body"`
	AnnouncementURL    string            `json:"announcement_url"`
	ContactURL         string            `json:"contact_url"`
	RegistrationPolicy string            `json:"registration_policy"`
	StartsAt           int64             `json:"starts_at"`
	EndsAt             int64             `json:"ends_at"`
	Access             SubsiteAccessInfo `json:"access"`
}

type SubsiteMemberInfo struct {
	SubsiteId int64  `json:"subsite_id"`
	UserId    int    `json:"user_id"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CanAccess bool   `json:"can_access"`
	CanManage bool   `json:"can_manage"`
}

type SubsiteTokenInfo struct {
	Id             int    `json:"id"`
	Name           string `json:"name"`
	Key            string `json:"key,omitempty"`
	MaskedKey      string `json:"masked_key"`
	Status         int    `json:"status"`
	CreatedTime    int64  `json:"created_time"`
	AccessedTime   int64  `json:"accessed_time"`
	ExpiredTime    int64  `json:"expired_time"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
}

type SubsiteQuotaMetric struct {
	Limit         int   `json:"limit"`
	Used          int   `json:"used"`
	Remaining     int   `json:"remaining"`
	WindowStart   int64 `json:"window_start"`
	WindowEnd     int64 `json:"window_end"`
	NextResetTime int64 `json:"next_reset_time"`
	WindowSeconds int64 `json:"window_seconds"`
}

type SubsiteQuotaSummary struct {
	SiteDailyQuota     SubsiteQuotaMetric `json:"site_daily_quota"`
	SiteWindowQuota    SubsiteQuotaMetric `json:"site_window_quota"`
	UserDailyQuota     SubsiteQuotaMetric `json:"user_daily_quota"`
	UserWindowQuota    SubsiteQuotaMetric `json:"user_window_quota"`
	SiteDailyRequests  SubsiteQuotaMetric `json:"site_daily_requests"`
	SiteWindowRequests SubsiteQuotaMetric `json:"site_window_requests"`
	UserDailyRequests  SubsiteQuotaMetric `json:"user_daily_requests"`
	UserWindowRequests SubsiteQuotaMetric `json:"user_window_requests"`
}

type SubsiteUsageStats struct {
	WindowSeconds int64 `json:"window_seconds"`
	Calls         int64 `json:"calls"`
	PromptTokens  int64 `json:"prompt_tokens"`
	OutputTokens  int64 `json:"output_tokens"`
	TotalTokens   int64 `json:"total_tokens"`
	Quota         int64 `json:"quota"`
	LastRequestAt int64 `json:"last_request_at"`
}

type SubsiteRecentLog struct {
	Id               int    `json:"id"`
	CreatedAt        int64  `json:"created_at"`
	Type             int    `json:"type"`
	Username         string `json:"username"`
	ModelName        string `json:"model_name"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	CacheTokens      int    `json:"cache_tokens"`
	ReasoningTokens  int    `json:"reasoning_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Quota            int    `json:"quota"`
	UseTime          int    `json:"use_time"`
	Status           string `json:"status"`
}

type SubsiteDashboard struct {
	Subsite    PublicSubsite       `json:"subsite"`
	Member     SubsiteMemberInfo   `json:"member"`
	BaseURL    string              `json:"base_url"`
	Token      *SubsiteTokenInfo   `json:"token"`
	Quota      SubsiteQuotaSummary `json:"quota"`
	Stats24h   SubsiteUsageStats   `json:"stats_24h"`
	RecentLogs []SubsiteRecentLog  `json:"recent_logs"`
}

type ManagedSubsite struct {
	Subsite        PublicSubsite           `json:"subsite"`
	Role           string                  `json:"role"`
	CanManage      bool                    `json:"can_manage"`
	OwnerUserIds   []int                   `json:"owner_user_ids"`
	OwnerUsernames []string                `json:"owner_usernames"`
	MemberCount    int64                   `json:"member_count"`
	TodayCalls     int64                   `json:"today_calls"`
	TodayQuota     int64                   `json:"today_quota"`
	QuotaPolicy    *SubsiteQuotaPolicyInfo `json:"quota_policy,omitempty"`
}

type ManagedSubsiteMemberInfo struct {
	Id          int64  `json:"id"`
	SubsiteId   int64  `json:"subsite_id"`
	UserId      int    `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	UserStatus  int    `json:"user_status"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CanAccess   bool   `json:"can_access"`
	CanManage   bool   `json:"can_manage"`
	JoinedAt    int64  `json:"joined_at"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type ManagedSubsiteActivity struct {
	Stats24h      SubsiteUsageStats  `json:"stats_24h"`
	ErrorCalls24h int64              `json:"error_calls_24h"`
	RecentLogs    []SubsiteRecentLog `json:"recent_logs"`
}

type ManagedSubsiteChannelInfo struct {
	Id                int               `json:"id"`
	SubsiteId         int64             `json:"subsite_id"`
	Name              string            `json:"name"`
	Type              int               `json:"type"`
	Status            int               `json:"status"`
	Models            string            `json:"models"`
	Group             string            `json:"group"`
	BaseURL           string            `json:"base_url"`
	Priority          int64             `json:"priority"`
	Weight            uint              `json:"weight"`
	CreatedTime       int64             `json:"created_time"`
	TestTime          int64             `json:"test_time"`
	ResponseTime      int               `json:"response_time"`
	UsedQuota         int64             `json:"used_quota"`
	Balance           float64           `json:"balance"`
	Remark            string            `json:"remark"`
	HasKey            bool              `json:"has_key"`
	ModelDisplayNames map[string]string `json:"model_display_names"`
}

type ManagedSubsiteChannelUpsertRequest struct {
	Name              string            `json:"name"`
	Type              int               `json:"type"`
	Key               string            `json:"key"`
	BaseURL           string            `json:"base_url"`
	Models            string            `json:"models"`
	Group             string            `json:"group"`
	Status            int               `json:"status"`
	Priority          int64             `json:"priority"`
	Weight            uint              `json:"weight"`
	Remark            string            `json:"remark"`
	ModelDisplayNames map[string]string `json:"model_display_names"`
}

type SubsiteQuotaPolicyInfo struct {
	SiteDailyQuota         int   `json:"site_daily_quota"`
	SiteWindowQuota        int   `json:"site_window_quota"`
	UserDailyQuota         int   `json:"user_daily_quota"`
	UserWindowQuota        int   `json:"user_window_quota"`
	SiteDailyRequestLimit  int   `json:"site_daily_request_limit"`
	SiteWindowRequestLimit int   `json:"site_window_request_limit"`
	UserDailyRequestLimit  int   `json:"user_daily_request_limit"`
	UserWindowRequestLimit int   `json:"user_window_request_limit"`
	SiteWindowSeconds      int64 `json:"site_window_seconds"`
	UserWindowSeconds      int64 `json:"user_window_seconds"`
}

type ManagedSubsiteCreateRequest struct {
	Slug                 string                  `json:"slug"`
	Name                 string                  `json:"name"`
	Title                string                  `json:"title"`
	LogoURL              string                  `json:"logo_url"`
	FaviconURL           string                  `json:"favicon_url"`
	ThemeColor           string                  `json:"theme_color"`
	Status               string                  `json:"status"`
	DisabledReason       string                  `json:"disabled_reason"`
	AnnouncementIcon     string                  `json:"announcement_icon"`
	AnnouncementTitle    string                  `json:"announcement_title"`
	AnnouncementBody     string                  `json:"announcement_body"`
	AnnouncementURL      string                  `json:"announcement_url"`
	ContactURL           string                  `json:"contact_url"`
	RegistrationPolicy   string                  `json:"registration_policy"`
	InviteCode           string                  `json:"invite_code"`
	EmailDomainWhitelist string                  `json:"email_domain_whitelist"`
	StartsAt             int64                   `json:"starts_at"`
	EndsAt               int64                   `json:"ends_at"`
	OwnerUserId          int                     `json:"owner_user_id"`
	QuotaPolicy          *SubsiteQuotaPolicyInfo `json:"quota_policy"`
}

type ManagedSubsiteMemberUpsertRequest struct {
	UserId int    `json:"user_id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type ManagedSubsiteUpdateRequest struct {
	Slug                 *string                 `json:"slug"`
	Name                 *string                 `json:"name"`
	Title                *string                 `json:"title"`
	LogoURL              *string                 `json:"logo_url"`
	FaviconURL           *string                 `json:"favicon_url"`
	ThemeColor           *string                 `json:"theme_color"`
	Status               *string                 `json:"status"`
	DisabledReason       *string                 `json:"disabled_reason"`
	AnnouncementIcon     *string                 `json:"announcement_icon"`
	AnnouncementTitle    *string                 `json:"announcement_title"`
	AnnouncementBody     *string                 `json:"announcement_body"`
	AnnouncementURL      *string                 `json:"announcement_url"`
	ContactURL           *string                 `json:"contact_url"`
	RegistrationPolicy   *string                 `json:"registration_policy"`
	InviteCode           *string                 `json:"invite_code"`
	EmailDomainWhitelist *string                 `json:"email_domain_whitelist"`
	StartsAt             *int64                  `json:"starts_at"`
	EndsAt               *int64                  `json:"ends_at"`
	QuotaPolicy          *SubsiteQuotaPolicyInfo `json:"quota_policy"`
}
