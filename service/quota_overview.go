package service

import (
	"errors"

	"github.com/QuantumNous/new-api/model"
)

var errInvalidQuotaOverviewUser = errors.New("invalid user id")

// QuotaOverview is the unified user-facing balance summary.
// Units are intentionally NOT merged:
//   - wallet / subscription / api_key: system quota points (money-like)
//   - model_token_package: LLM token counts
type QuotaOverview struct {
	Wallet             QuotaOverviewWallet             `json:"wallet"`
	ModelTokenPackages QuotaOverviewModelTokenPackages `json:"model_token_packages"`
	Subscriptions      QuotaOverviewSubscriptions      `json:"subscriptions"`
	APIKeyHardLimits   QuotaOverviewAPIKeyHardLimits   `json:"api_key_hard_limits"`
	Units              QuotaOverviewUnits              `json:"units"`
	Links              QuotaOverviewLinks              `json:"links"`
}

type QuotaOverviewUnits struct {
	Wallet            string `json:"wallet"`              // "quota_points"
	ModelTokenPackage string `json:"model_token_package"` // "llm_tokens"
	Subscription      string `json:"subscription"`        // "quota_points"
	APIKeyHardLimit   string `json:"api_key_hard_limit"`  // "quota_points"
}

type QuotaOverviewLinks struct {
	Wallet             string `json:"wallet"`
	ModelTokenPackages string `json:"model_token_packages"`
	Subscriptions      string `json:"subscriptions"`
	APIKeys            string `json:"api_keys"`
}

type QuotaOverviewWallet struct {
	Quota        int    `json:"quota"`
	UsedQuota    int    `json:"used_quota"`
	RequestCount int    `json:"request_count"`
	Unit         string `json:"unit"`
	Status       string `json:"status"` // ok | empty
}

type QuotaOverviewModelTokenPackages struct {
	ActiveCount     int                        `json:"active_count"`
	RemainingTokens int64                      `json:"remaining_tokens"`
	UsedTokens      int64                      `json:"used_tokens"`
	TotalPackages   int                        `json:"total_packages"`
	Unit            string                     `json:"unit"`
	Status          string                     `json:"status"` // ok | empty
	TopPackages     []QuotaOverviewPackageItem `json:"top_packages"`
}

type QuotaOverviewPackageItem struct {
	Id              int      `json:"id"`
	Name            string   `json:"name"`
	Models          []string `json:"models"`
	RemainingTokens int64    `json:"remaining_tokens"`
	TotalTokens     int64    `json:"total_tokens"`
	Status          string   `json:"status"`
	ExpiredAt       int64    `json:"expired_at"`
}

type QuotaOverviewSubscriptions struct {
	ActiveCount    int                             `json:"active_count"`
	RemainingQuota int64                           `json:"remaining_quota"`
	TotalQuota     int64                           `json:"total_quota"`
	UsedQuota      int64                           `json:"used_quota"`
	Unit           string                          `json:"unit"`
	Status         string                          `json:"status"` // ok | empty
	Items          []QuotaOverviewSubscriptionItem `json:"items"`
}

type QuotaOverviewSubscriptionItem struct {
	Id              int    `json:"id"`
	PlanId          int    `json:"plan_id"`
	AmountTotal     int64  `json:"amount_total"`
	AmountUsed      int64  `json:"amount_used"`
	AmountRemaining int64  `json:"amount_remaining"`
	Status          string `json:"status"`
	EndTime         int64  `json:"end_time"`
	Source          string `json:"source"`
}

type QuotaOverviewAPIKeyHardLimits struct {
	LimitedCount   int                       `json:"limited_count"`
	RemainingQuota int64                     `json:"remaining_quota"`
	Unit           string                    `json:"unit"`
	Status         string                    `json:"status"` // ok | empty
	Items          []QuotaOverviewAPIKeyItem `json:"items"`
}

type QuotaOverviewAPIKeyItem struct {
	Id                    int    `json:"id"`
	Name                  string `json:"name"`
	RemainQuota           int    `json:"remain_quota"`
	UsedQuota             int    `json:"used_quota"`
	UnlimitedQuota        bool   `json:"unlimited_quota"`
	QuotaHardLimitEnabled bool   `json:"quota_hard_limit_enabled"`
	Status                int    `json:"status"`
	ExpiredTime           int64  `json:"expired_time"`
}

// BuildUserQuotaOverview aggregates wallet, packages, subscriptions, and limited API keys.
func BuildUserQuotaOverview(userId int) (*QuotaOverview, error) {
	if userId <= 0 {
		return nil, errInvalidQuotaOverviewUser
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return nil, err
	}

	overview := &QuotaOverview{
		Units: QuotaOverviewUnits{
			Wallet:            "quota_points",
			ModelTokenPackage: "llm_tokens",
			Subscription:      "quota_points",
			APIKeyHardLimit:   "quota_points",
		},
		Links: QuotaOverviewLinks{
			Wallet:             "/wallet",
			ModelTokenPackages: "/wallet#model-token-packages",
			Subscriptions:      "/wallet#subscriptions",
			APIKeys:            "/keys",
		},
		Wallet: QuotaOverviewWallet{
			Quota:        user.Quota,
			UsedQuota:    user.UsedQuota,
			RequestCount: user.RequestCount,
			Unit:         "quota_points",
			Status:       "ok",
		},
	}
	if user.Quota <= 0 {
		overview.Wallet.Status = "empty"
	}

	// Packages
	if err := model.FillUserModelTokenPackageSummaries([]*model.User{user}); err != nil {
		return nil, err
	}
	packages, err := model.ListModelTokenPackagesByUser(userId, true)
	if err != nil {
		return nil, err
	}
	overview.ModelTokenPackages = QuotaOverviewModelTokenPackages{
		ActiveCount:     user.ModelTokenPackageActiveCount,
		RemainingTokens: user.ModelTokenPackageRemaining,
		UsedTokens:      user.ModelTokenPackageUsed,
		TotalPackages:   user.ModelTokenPackageTotal,
		Unit:            "llm_tokens",
		Status:          "ok",
		TopPackages:     make([]QuotaOverviewPackageItem, 0, 5),
	}
	if user.ModelTokenPackageTotal == 0 {
		overview.ModelTokenPackages.Status = "empty"
	}
	// Prefer active packages first in top list
	activeFirst := make([]model.ModelTokenPackage, 0, len(packages))
	rest := make([]model.ModelTokenPackage, 0, len(packages))
	for _, pkg := range packages {
		if pkg.Status == model.ModelTokenPackageStatusActive {
			activeFirst = append(activeFirst, pkg)
		} else {
			rest = append(rest, pkg)
		}
	}
	ordered := append(activeFirst, rest...)
	for i, pkg := range ordered {
		if i >= 5 {
			break
		}
		overview.ModelTokenPackages.TopPackages = append(overview.ModelTokenPackages.TopPackages, QuotaOverviewPackageItem{
			Id:              pkg.Id,
			Name:            pkg.Name,
			Models:          pkg.Models,
			RemainingTokens: pkg.RemainingTokens,
			TotalTokens:     pkg.TotalTokens,
			Status:          pkg.Status,
			ExpiredAt:       pkg.ExpiredAt,
		})
	}

	// Subscriptions
	subs, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		return nil, err
	}
	overview.Subscriptions = QuotaOverviewSubscriptions{
		ActiveCount: len(subs),
		Unit:        "quota_points",
		Status:      "ok",
		Items:       make([]QuotaOverviewSubscriptionItem, 0, len(subs)),
	}
	if len(subs) == 0 {
		overview.Subscriptions.Status = "empty"
	}
	for _, summary := range subs {
		if summary.Subscription == nil {
			continue
		}
		sub := summary.Subscription
		remaining := sub.AmountTotal - sub.AmountUsed
		if remaining < 0 {
			remaining = 0
		}
		overview.Subscriptions.TotalQuota += sub.AmountTotal
		overview.Subscriptions.UsedQuota += sub.AmountUsed
		overview.Subscriptions.RemainingQuota += remaining
		overview.Subscriptions.Items = append(overview.Subscriptions.Items, QuotaOverviewSubscriptionItem{
			Id:              sub.Id,
			PlanId:          sub.PlanId,
			AmountTotal:     sub.AmountTotal,
			AmountUsed:      sub.AmountUsed,
			AmountRemaining: remaining,
			Status:          sub.Status,
			EndTime:         sub.EndTime,
			Source:          sub.Source,
		})
	}

	// API keys with hard limits (cap list size)
	tokens, err := model.GetAllUserTokens(userId, 0, 200)
	if err != nil {
		return nil, err
	}
	overview.APIKeyHardLimits = QuotaOverviewAPIKeyHardLimits{
		Unit:   "quota_points",
		Status: "ok",
		Items:  make([]QuotaOverviewAPIKeyItem, 0),
	}
	var remainingSum int64
	for _, token := range tokens {
		if token == nil || !token.IsQuotaLimited() {
			continue
		}
		overview.APIKeyHardLimits.LimitedCount++
		remainingSum += int64(token.RemainQuota)
		if len(overview.APIKeyHardLimits.Items) < 20 {
			overview.APIKeyHardLimits.Items = append(overview.APIKeyHardLimits.Items, QuotaOverviewAPIKeyItem{
				Id:                    token.Id,
				Name:                  token.Name,
				RemainQuota:           token.RemainQuota,
				UsedQuota:             token.UsedQuota,
				UnlimitedQuota:        token.UnlimitedQuota,
				QuotaHardLimitEnabled: token.QuotaHardLimitEnabled,
				Status:                token.Status,
				ExpiredTime:           token.ExpiredTime,
			})
		}
	}
	overview.APIKeyHardLimits.RemainingQuota = remainingSum
	if overview.APIKeyHardLimits.LimitedCount == 0 {
		overview.APIKeyHardLimits.Status = "empty"
	}

	return overview, nil
}
