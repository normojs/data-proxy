package billing

import (
	"fmt"

	"github.com/QuantumNous/new-api/model"
)

type SettleInput struct {
	CallId                     int64
	UserId                     int
	TokenId                    int
	TokenUnlimited             bool
	TokenQuotaHardLimitEnabled bool
	PriceUnit                  string
	Result                     PrecheckResult
}

type SettleResult struct {
	Quota   int  `json:"quota"`
	Settled bool `json:"settled"`
	Free    bool `json:"free"`
}

func Settle(input SettleInput) (SettleResult, error) {
	result := SettleResult{
		Quota: input.Result.Quota,
		Free:  input.Result.FreeUsed || input.Result.Quota <= 0,
	}
	if input.CallId <= 0 {
		return result, fmt.Errorf("invalid MCP tool call id: %d", input.CallId)
	}
	if input.UserId <= 0 {
		return result, fmt.Errorf("invalid MCP user id: %d", input.UserId)
	}
	settled, err := model.SettleMCPToolCallQuota(
		input.CallId,
		input.UserId,
		input.TokenId,
		result.Quota,
		input.Result.Cost,
		input.TokenUnlimited,
		input.PriceUnit,
		input.TokenQuotaHardLimitEnabled,
	)
	if err != nil {
		return result, err
	}
	result.Settled = settled
	return result, nil
}
