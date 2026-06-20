package billing

import (
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type PrecheckInput struct {
	UserId                     int
	TokenId                    int
	TokenKey                   string
	TokenUnlimited             bool
	TokenQuotaHardLimitEnabled bool
	TokenQuota                 int
	UsingGroup                 string
	PricePerCall               float64
	PriceUnit                  string
	FreeQuotaActive            bool
}

type PrecheckResult struct {
	Cost       float64 `json:"cost"`
	Quota      int     `json:"quota"`
	GroupRatio float64 `json:"group_ratio"`
	PriceUnit  string  `json:"price_unit"`
	FreeUsed   bool    `json:"free_used"`
}

type PrecheckError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Result  PrecheckResult `json:"result"`
}

func (e *PrecheckError) Error() string {
	return e.Message
}

func Precheck(input PrecheckInput) (PrecheckResult, error) {
	result := Estimate(input)
	if result.FreeUsed || result.Quota <= 0 {
		return result, nil
	}

	userQuota, err := model.GetUserQuota(input.UserId, false)
	if err != nil {
		return result, err
	}
	if userQuota < result.Quota {
		return result, &PrecheckError{
			Code:    "INSUFFICIENT_USER_QUOTA",
			Message: fmt.Sprintf("user quota is insufficient, remaining=%d required=%d", userQuota, result.Quota),
			Result:  result,
		}
	}

	if (!input.TokenUnlimited || input.TokenQuotaHardLimitEnabled) && input.TokenQuota < result.Quota {
		return result, &PrecheckError{
			Code:    "INSUFFICIENT_TOKEN_QUOTA",
			Message: fmt.Sprintf("token quota is insufficient, remaining=%d required=%d", input.TokenQuota, result.Quota),
			Result:  result,
		}
	}

	return result, nil
}

func Estimate(input PrecheckInput) PrecheckResult {
	group := input.UsingGroup
	if group == "" {
		group = "default"
	}
	groupRatio := ratio_setting.GetGroupRatio(group)
	if groupRatio < 0 {
		groupRatio = 1
	}

	if input.FreeQuotaActive {
		return PrecheckResult{
			Cost:       0,
			Quota:      0,
			GroupRatio: groupRatio,
			PriceUnit:  model.MCPToolPriceUnitPerCall,
			FreeUsed:   true,
		}
	}

	cost := input.PricePerCall
	quota := int(math.Round(cost * common.QuotaPerUnit * groupRatio))
	if quota < 0 {
		quota = 0
	}
	return PrecheckResult{
		Cost:       cost,
		Quota:      quota,
		GroupRatio: groupRatio,
		PriceUnit:  model.MCPToolPriceUnitPerCall,
	}
}
